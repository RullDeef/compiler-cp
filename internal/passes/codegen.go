package passes

import (
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/typesystem"
	"gocomp/internal/utils"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type CodeGenVisitor struct {
	parser.BaseGoParserVisitor
	packageData *PackageData
	genCtx      *GenContext

	// unique index per function body generation
	UID int

	currentFuncDecl *FunctionDecl
	currentFuncIR   *ir.Func

	loopStack    []loopBlocks // break + continue
	labelManager              // goto handling
	deferManager              // defer handling
	*typeManager              // user defined types handling
}

func NewCodeGenVisitor(pdata *PackageData) (*CodeGenVisitor, error) {
	genCtx, err := NewGenContext(pdata)
	if err != nil {
		return nil, err
	}
	return &CodeGenVisitor{
		packageData: pdata,
		genCtx:      genCtx,
		typeManager: pdata.typeManager,
	}, nil
}

func (v *CodeGenVisitor) VisitSourceFile(ctx parser.ISourceFileContext) (*ir.Module, error) {
	// gather global declarations
	ctorFun := ir.NewFunc(fmt.Sprintf("%s_init", v.packageData.PackageName), types.Void)
	globalInitBlocks := []*ir.Block{ir.NewBlock("entry")}
	for _, decl := range ctx.AllDeclaration() {
		blocks, err := v.VisitDeclaration(globalInitBlocks[len(globalInitBlocks)-1], true, decl)
		if err != nil {
			return nil, err
		} else if blocks != nil {
			globalInitBlocks = append(globalInitBlocks, blocks...)
		}
	}
	globalInitBlocks[len(globalInitBlocks)-1].NewRet(nil)
	ctorFun.Blocks = globalInitBlocks

	// add code for each function declaration
	for _, fun := range ctx.AllFunctionDecl() {
		res := v.VisitFunctionDecl(fun.(*parser.FunctionDeclContext))
		if res != nil {
			return nil, utils.MakeError("failed to parse func %s: %w", fun.IDENTIFIER().GetText(), res.(error))
		}
	}

	// build real main function
	module := v.genCtx.Module()

	// update type defs
	v.typeManager.UpdateModule(module)

	var mainFun *ir.Func
	for _, fun := range module.Funcs {
		if fun.Name() == v.packageData.PackageName+"__main" {
			mainFun = fun
			break
		}
	}
	if mainFun == nil {
		return nil, utils.MakeError("main function not found")
	}
	module.Funcs = append(module.Funcs, ctorFun)
	realMainFun := module.NewFunc("main", types.I32)
	realMainEntry := realMainFun.NewBlock("entry")
	realMainEntry.NewCall(ctorFun)
	realMainEntry.NewCall(mainFun)
	realMainEntry.NewRet(constant.NewInt(types.I32, 0))

	return module, nil
}

func (v *CodeGenVisitor) VisitDeclaration(block *ir.Block, globalScope bool, ctx parser.IDeclarationContext) ([]*ir.Block, error) {
	var newBlocks []*ir.Block
	// populate global consts and variables
	if ctx.ConstDecl() != nil {
		for _, spec := range ctx.ConstDecl().AllConstSpec() {
			blocks, err := v.VisitConstSpec(block, globalScope, spec)
			if err != nil {
				return nil, err
			} else if blocks != nil {
				newBlocks = append(newBlocks, blocks...)
				block = newBlocks[len(newBlocks)-1]
			}
		}
		return newBlocks, nil
	} else if ctx.VarDecl() != nil {
		for _, spec := range ctx.VarDecl().AllVarSpec() {
			blocks, err := v.VisitVarSpec(block, globalScope, spec)
			if err != nil {
				return nil, err
			} else if blocks != nil {
				newBlocks = append(newBlocks, blocks...)
				block = newBlocks[len(newBlocks)-1]
			}
		}
		return newBlocks, nil
	} else if ctx.TypeDecl() != nil {
		// err := v.ParseTypeDecl(ctx.TypeDecl())
		// return nil, err
		return nil, nil
	} else {
		panic("must never happen")
	}
}

func (v *CodeGenVisitor) VisitConstSpec(block *ir.Block, globalScope bool, ctx parser.IConstSpecContext) ([]*ir.Block, error) {
	// iota and inherited declarations not supported yet
	blocks, ids, vals, err := v.VisitConstVarSpec(block, ctx)
	if err != nil {
		return nil, err
	}
	for i := range ids {
		// global consts only
		var memRef value.Value
		if globalScope {
			glob := v.genCtx.module.NewGlobal(ids[i], vals[i].Type())
			glob.Init = &constant.ZeroInitializer{}
			// glob.Immutable = true
			memRef = glob
		} else {
			memRef = block.NewAlloca(vals[i].Type())
		}
		block.NewStore(vals[i], memRef)
		if err := v.genCtx.Vars.Add(ids[i], memRef); err != nil {
			return nil, err
		}
	}
	return blocks, nil
}

func (v *CodeGenVisitor) VisitVarSpec(block *ir.Block, globalScope bool, ctx parser.IVarSpecContext) ([]*ir.Block, error) {
	blocks, ids, vals, err := v.VisitConstVarSpec(block, ctx)
	if err != nil {
		return nil, err
	}
	for i := range ids {
		var memRef value.Value
		if globalScope {
			glob := v.genCtx.module.NewGlobal(ids[i], vals[i].Type())
			glob.Init = constant.NewZeroInitializer(vals[i].Type())
			memRef = glob
		} else {
			memRef = block.NewAlloca(vals[i].Type())
		}
		block.NewStore(vals[i], memRef)
		if err := v.genCtx.Vars.Add(ids[i], memRef); err != nil {
			return nil, err
		}
	}
	return blocks, nil
}

type ConstVarContext interface {
	IdentifierList() parser.IIdentifierListContext
	ExpressionList() parser.IExpressionListContext
	Type_() parser.IType_Context
}

func (v *CodeGenVisitor) VisitConstVarSpec(block *ir.Block, ctx ConstVarContext) ([]*ir.Block, []string, []value.Value, error) {
	// iota and inherited declarations not supported yet
	ids := v.genCtx.GenerateIdentList(ctx.IdentifierList())
	var vals []value.Value
	var blocks []*ir.Block
	if ctx.ExpressionList() != nil {
		var err error
		vals, blocks, err = v.genCtx.GenerateExprList(block, ctx.ExpressionList())
		if err != nil {
			return nil, nil, nil, err
		}
	} else if ctx.Type_() != nil {
		// zero value init based on type
		llvmType, err := v.ParseType(ctx.Type_())
		if err != nil {
			return nil, nil, nil, err
		}
		for range ids {
			vals = append(vals, constant.NewZeroInitializer(llvmType))
		}
	} else {
		// invalid situation
		return nil, nil, nil, utils.MakeError("invalid declaration spec")
	}
	if len(ids) != len(vals) {
		return nil, nil, nil, utils.MakeError("umatched count of ids(%d) and vals(%d) in declaration spec", len(ids), len(vals))
	}
	return blocks, ids, vals, nil
}

func (v *CodeGenVisitor) VisitFunctionDecl(ctx parser.IFunctionDeclContext) interface{} {
	fun, err := v.genCtx.LookupFunc(ctx.IDENTIFIER().GetText())
	if err != nil {
		return err
	}

	v.currentFuncDecl = v.packageData.Functions[fun.Name()]
	v.currentFuncIR = fun

	// setup local var storage
	v.genCtx.PushLexicalScope()
	defer v.genCtx.PopLexicalScope()

	v.UID = 0

	// populate function arguments
	block := fun.NewBlock("entry")
	for i, param := range fun.Params {
		if i < len(v.currentFuncDecl.ReturnTypes) && len(v.currentFuncDecl.ReturnTypes) > 1 {
			// out parameter
			v.genCtx.Vars.Add(param.Name(), param)
		} else {
			// regular parameter
			memRef := block.NewAlloca(param.Type())
			block.NewStore(param, memRef)
			v.genCtx.Vars.Add(param.Name(), memRef)
		}
	}

	// initialize & cleanup goto labels
	v.labelManager.clearLabels()
	defer v.labelManager.clearLabels()

	// codegen body
	bodyBlocks, err := v.VisitBlock(block, ctx.Block())
	if err != nil {
		return utils.MakeError("failed to parse body: %w", err)
	} else {
		for _, block := range bodyBlocks {
			block.Parent = fun
		}
		fun.Blocks = append(fun.Blocks, bodyBlocks...)
		bodyBlocks = append([]*ir.Block{block}, bodyBlocks...)
		if bodyBlocks[len(bodyBlocks)-1].Term == nil {
			// add void return stmt
			block = bodyBlocks[len(bodyBlocks)-1]
			v.deferManager.applyDefers(block)
			block.NewRet(nil)
		}
		return v.labelManager.checkLabelsDefined()
	}
}

func (v *CodeGenVisitor) VisitBlock(block *ir.Block, ctx parser.IBlockContext) ([]*ir.Block, error) {
	v.genCtx.PushLexicalScope()
	defer v.genCtx.PopLexicalScope()

	var blocks []*ir.Block
	if ctx.StatementList() != nil {
		for _, stmt := range ctx.StatementList().AllStatement() {
			if newBlocks, err := v.VisitStatement(block, stmt); err != nil {
				return nil, utils.MakeError("failed to parse statement: %w", err)
			} else if newBlocks != nil {
				blocks = append(blocks, newBlocks...)
				block = blocks[len(blocks)-1]
			}
		}
	}
	return blocks, nil
}

func (v *CodeGenVisitor) VisitStatement(block *ir.Block, ctx parser.IStatementContext) ([]*ir.Block, error) {
	switch s := ctx.GetChild(0).(type) {
	case parser.ISimpleStmtContext:
		return v.VisitSimpleStatement(block, s)
	case parser.IReturnStmtContext:
		return v.VisitReturnStmt(block, s)
	case parser.IIfStmtContext:
		return v.VisitIfStmt(block, s)
	case parser.IBlockContext:
		return v.VisitBlock(block, s)
	case parser.IForStmtContext:
		return v.VisitForStmt(block, s)
	case parser.IDeclarationContext:
		return v.VisitDeclaration(block, false, s)
	case parser.IBreakStmtContext:
		return nil, v.VisitBreakStmt(block, s)
	case parser.IContinueStmtContext:
		return nil, v.VisitContinueStmt(block, s)
	case parser.ILabeledStmtContext:
		return v.VisitLabeledStmt(block, s)
	case parser.IGotoStmtContext:
		return v.VisitGotoStmt(block, s)
	case parser.IDeferStmtContext:
		return v.VisitDeferStmt(block, s)
	default:
		return nil, utils.MakeError("unsupported instruction")
	}
}

func (v *CodeGenVisitor) VisitIfStmt(block *ir.Block, ctx parser.IIfStmtContext) ([]*ir.Block, error) {
	if ctx.SimpleStmt() != nil {
		return nil, utils.MakeError("unsupported init statement in if")
	}
	exprs, newBlocks, err := v.genCtx.GenerateExpr(block, ctx.Expression())
	if err != nil {
		return nil, utils.MakeError("failed to parse if expression: %w", err)
	} else if !typesystem.IsBoolType(exprs[0].Type()) {
		return nil, utils.MakeError("expression must have boolean type")
	} else if newBlocks != nil {
		block = newBlocks[len(newBlocks)-1]
	}
	stmtUID := v.UID
	v.UID++
	btrue := ir.NewBlock(fmt.Sprintf("btrue.%d", stmtUID))
	bfalse := ir.NewBlock(fmt.Sprintf("bfalse.%d", stmtUID))
	block.NewCondBr(exprs[0], btrue, bfalse)

	newBlocks = append(newBlocks, btrue)
	trueBlocks, err := v.VisitBlock(btrue, ctx.Block(0))
	if err != nil {
		return nil, err
	} else if trueBlocks != nil {
		newBlocks = append(newBlocks, trueBlocks...)
		btrue = newBlocks[len(newBlocks)-1]
	}

	newBlocks = append(newBlocks, bfalse)
	if ctx.ELSE() != nil {
		var blocks []*ir.Block
		var err error
		if ctx.IfStmt() != nil {
			blocks, err = v.VisitIfStmt(bfalse, ctx.IfStmt())
		} else {
			blocks, err = v.VisitBlock(bfalse, ctx.Block(1))
		}
		if err != nil {
			return nil, err
		} else if blocks != nil {
			newBlocks = append(newBlocks, blocks...)
			bfalse = newBlocks[len(newBlocks)-1]
		}
	}

	// end block
	if btrue.Term == nil || bfalse.Term == nil {
		bend := ir.NewBlock(fmt.Sprintf("bend.%d", stmtUID))
		if btrue.Term == nil {
			btrue.NewBr(bend)
		}
		if bfalse.Term == nil {
			bfalse.NewBr(bend)
		}
		newBlocks = append(newBlocks, bend)
	}

	return newBlocks, nil
}

func (v *CodeGenVisitor) VisitSimpleStatement(block *ir.Block, ctx parser.ISimpleStmtContext) ([]*ir.Block, error) {
	switch s := ctx.GetChild(0).(type) {
	case parser.IAssignmentContext:
		return v.VisitAssignment(block, s)
	case parser.IShortVarDeclContext:
		return v.VisitShortVarDecl(block, s)
	case parser.IExpressionStmtContext:
		_, blocks, err := v.genCtx.GenerateExpr(block, s.Expression())
		return blocks, err
	case parser.IIncDecStmtContext:
		return v.VisitIncDecStmt(block, s)
	default:
		return nil, utils.MakeError("unimplemented simple statement")
	}
}

func (v *CodeGenVisitor) VisitIncDecStmt(block *ir.Block, ctx parser.IIncDecStmtContext) ([]*ir.Block, error) {
	// assume variable name in expression
	varName := ctx.Expression().GetText()
	varRef, ok := v.genCtx.Vars.Lookup(varName)
	if !ok {
		return nil, utils.MakeError("variable %s not found in this scope", varName)
	}
	itype, ok := varRef.Type().(*types.PointerType).ElemType.(*types.IntType)
	if !ok {
		return nil, utils.MakeError("variable %s is not of integer type", varName)
	}
	varVal := block.NewLoad(varRef.Type().(*types.PointerType).ElemType, varRef)
	var res value.Value
	if ctx.PLUS_PLUS() != nil {
		elType := varRef.Type().(*types.PointerType).ElemType
		res = block.NewAdd(varVal, constant.NewInt(elType.(*types.IntType), 1))
	} else {
		res = block.NewSub(varVal, constant.NewInt(itype, 1))
	}
	block.NewStore(res, varRef)
	return nil, nil
}

func (v *CodeGenVisitor) VisitAssignment(block *ir.Block, ctx parser.IAssignmentContext) ([]*ir.Block, error) {
	lvals, newBlocks, err := v.genCtx.GenerateLValueList(block, ctx.ExpressionList(0))
	if err != nil {
		return nil, err
	} else if newBlocks != nil {
		block = newBlocks[len(newBlocks)-1]
	}
	rvals, blocks, err := v.genCtx.GenerateExprList(block, ctx.ExpressionList(1))
	if err != nil {
		return nil, err
	} else if blocks != nil {
		newBlocks = append(newBlocks, blocks...)
		block = newBlocks[len(newBlocks)-1]
	}
	if len(lvals) != len(rvals) {
		return nil, utils.MakeError("unmatched lvals(%d) and rvals(%d) count", len(lvals), len(rvals))
	}
	for i := range len(rvals) {
		if lvals[i] != nil {
			block.NewStore(rvals[i], lvals[i])
		}
	}
	return nil, nil
}

func (v *CodeGenVisitor) VisitShortVarDecl(block *ir.Block, ctx parser.IShortVarDeclContext) ([]*ir.Block, error) {
	vals, blocks, err := v.genCtx.GenerateExprList(block, ctx.ExpressionList())
	if err != nil {
		return nil, err
	} else if blocks != nil {
		block = blocks[len(blocks)-1]
	}
	ids := v.genCtx.GenerateIdentList(ctx.IdentifierList())
	for i, val := range vals {
		varName := ids[i]
		if varName == "_" {
			continue
		}
		memRef := block.NewAlloca(val.Type())
		if err := v.genCtx.Vars.Add(varName, memRef); err != nil {
			return nil, err
		}
		block.NewStore(val, memRef)
	}
	return blocks, nil
}

func (v *CodeGenVisitor) VisitReturnStmt(block *ir.Block, ctx parser.IReturnStmtContext) ([]*ir.Block, error) {
	// TODO: return multiple values from function
	v.deferManager.applyDefers(block)
	if ctx.ExpressionList() == nil {
		block.NewRet(nil)
		return nil, nil
	}
	vals, newBlocks, err := v.genCtx.GenerateExprList(block, ctx.ExpressionList())
	if err != nil {
		return nil, err
	} else if newBlocks != nil {
		block = newBlocks[len(newBlocks)-1]
	}
	// match return types of function with value types
	if len(vals) == 1 {
		block.NewRet(vals[0])
	} else if len(vals) > 1 {
		// store result values in first func params (which are out params now)
		for i, val := range vals {
			pName := v.currentFuncIR.Params[i].Name()
			outPar, ok := v.genCtx.Vars.Lookup(pName)
			if !ok {
				return nil, utils.MakeError("invalid function parameter: %s", pName)
			}
			block.NewStore(val, outPar)
		}
		block.NewRet(nil)
	}
	return newBlocks, nil
}
