package passes

import (
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/typesystem"
	"gocomp/internal/utils"

	"github.com/antlr4-go/antlr/v4"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type CodeGenVisitor struct {
	parser.BaseGoParserVisitor
	packageData *PackageData
	genCtx      *GenContext

	currentFuncDecl *FunctionDecl
	currentFuncIR   *ir.Func

	branchManager
	labelManager // goto handling
	deferManager // defer handling
	*typeManager // user defined types handling
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
	// build real main function
	module := v.genCtx.Module()
	ctorFun, err := v.buildCtorFunc(module, ctx)
	if err != nil {
		return nil, err
	}
	dtorFun, err := v.buildDtorFunc(ctx)
	if err != nil {
		return nil, err
	}

	// add code for each function declaration
	for _, fun := range ctx.AllFunctionDecl() {
		res := v.VisitFunctionDecl(fun.(*parser.FunctionDeclContext))
		if err, ok := res.(error); ok {
			return nil, utils.MakeErrorTrace(ctx, err, "failed to parse func %s", fun.IDENTIFIER().GetText())
		}
	}

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
	module.Funcs = append(module.Funcs, ctorFun, dtorFun)
	realMainFun := module.NewFunc("main", types.I32)
	realMainEntry := realMainFun.NewBlock("entry")
	realMainEntry.NewCall(ctorFun)
	realMainEntry.NewCall(mainFun)
	realMainEntry.NewCall(dtorFun)
	realMainEntry.NewRet(constant.NewInt(types.I32, 0))

	return module, nil
}

func (v *CodeGenVisitor) buildCtorFunc(module *ir.Module, ctx parser.ISourceFileContext) (*ir.Func, error) {
	// gather global declarations
	ctorFun := ir.NewFunc(fmt.Sprintf("%s_init", v.packageData.PackageName), types.Void)
	globalInitBlocks := []*ir.Block{ir.NewBlock("entry")}

	// initialize GC
	gcInitFun, err := v.genCtx.LookupFunc("GC_init")
	if err != nil {
		return nil, err
	}
	globalInitBlocks[0].NewCall(gcInitFun)

	// initialize defer stack
	v.deferManager.initDeferStack(module, globalInitBlocks[0])

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
	return ctorFun, nil
}

func (v *CodeGenVisitor) buildDtorFunc(ctx parser.ISourceFileContext) (*ir.Func, error) {
	dtorFun := ir.NewFunc(fmt.Sprintf("%s_cleanup", v.packageData.PackageName), types.Void)
	globalInitBlocks := []*ir.Block{ir.NewBlock("entry")}

	v.deferManager.cleanupDeferStack(v.genCtx.module, globalInitBlocks[0])

	globalInitBlocks[len(globalInitBlocks)-1].NewRet(nil)
	dtorFun.Blocks = globalInitBlocks
	return dtorFun, nil
}

func (v *CodeGenVisitor) VisitDeclaration(block *ir.Block, globalScope bool, ctx parser.IDeclarationContext) ([]*ir.Block, error) {
	var newBlocks []*ir.Block
	// populate global consts and variables
	if ctx.ConstDecl() != nil {
		// iota and inherited declarations not supported yet
		for _, spec := range ctx.ConstDecl().AllConstSpec() {
			blocks, err := v.VisitConstVarSpecHelper(block, globalScope, spec)
			if err != nil {
				return nil, utils.MakeErrorTrace(spec, err, "failed to parse const declaration")
			} else if blocks != nil {
				newBlocks = append(newBlocks, blocks...)
				block = newBlocks[len(newBlocks)-1]
			}
		}
	} else if ctx.VarDecl() != nil {
		for _, spec := range ctx.VarDecl().AllVarSpec() {
			blocks, err := v.VisitConstVarSpecHelper(block, globalScope, spec)
			if err != nil {
				return nil, utils.MakeErrorTrace(spec, err, "failed to parse var declaration")
			} else if blocks != nil {
				newBlocks = append(newBlocks, blocks...)
				block = newBlocks[len(newBlocks)-1]
			}
		}
	}
	return newBlocks, nil
}

type ConstVarContext interface {
	antlr.ParserRuleContext
	IdentifierList() parser.IIdentifierListContext
	ExpressionList() parser.IExpressionListContext
	Type_() parser.IType_Context
}

func (v *CodeGenVisitor) VisitConstVarSpecHelper(block *ir.Block, globalScope bool, ctx ConstVarContext) ([]*ir.Block, error) {
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
		return nil, nil, nil, utils.MakeErrorTrace(ctx, nil, "invalid declaration spec")
	}
	if len(ids) != len(vals) {
		return nil, nil, nil, utils.MakeErrorTrace(ctx, nil, "umatched count of ids(%d) and vals(%d) in declaration spec", len(ids), len(vals))
	}
	return blocks, ids, vals, nil
}

func (v *CodeGenVisitor) VisitFunctionDecl(ctx parser.IFunctionDeclContext) interface{} {
	fun, err := v.genCtx.LookupFunc(ctx.IDENTIFIER().GetText())
	if err != nil {
		return utils.MakeErrorTrace(ctx, err, "failed to parse function declaration")
	}

	v.currentFuncDecl = v.packageData.Functions[fun.Name()]
	v.currentFuncIR = fun

	v.branchManager.EnterFuncDef()

	// setup local var storage
	v.genCtx.PushLexicalScope()
	defer v.genCtx.PopLexicalScope()

	// setup defer stack
	defer v.clearDeferStack()

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
		return utils.MakeErrorTrace(ctx, err, "failed to parse body")
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
				return nil, utils.MakeErrorTrace(stmt, err, "failed to parse statement")
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
		return nil, utils.MakeErrorTrace(ctx, nil, "unsupported instruction")
	}
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
		return nil, utils.MakeErrorTrace(ctx, nil, "unimplemented simple statement")
	}
}

func (v *CodeGenVisitor) VisitIncDecStmt(block *ir.Block, ctx parser.IIncDecStmtContext) ([]*ir.Block, error) {
	// assume variable name in expression
	varName := ctx.Expression().GetText()
	varRef, ok := v.genCtx.Vars.Lookup(varName)
	if !ok {
		return nil, utils.MakeErrorTrace(ctx, nil, "variable %s not found in this scope", varName)
	}
	itype, ok := varRef.Type().(*types.PointerType).ElemType.(*types.IntType)
	if !ok {
		return nil, utils.MakeErrorTrace(ctx, nil, "variable %s is not of integer type", varName)
	}
	varVal := block.NewLoad(varRef.Type().(*types.PointerType).ElemType, varRef)
	if ctx.PLUS_PLUS() != nil {
		block.NewStore(block.NewAdd(varVal, constant.NewInt(itype, 1)), varRef)
	} else {
		block.NewStore(block.NewSub(varVal, constant.NewInt(itype, 1)), varRef)
	}
	return nil, nil
}

func (v *CodeGenVisitor) VisitAssignment(block *ir.Block, ctx parser.IAssignmentContext) ([]*ir.Block, error) {
	lvals, newBlocks, err := v.genCtx.GenerateLValueList(block, ctx.ExpressionList(0))
	if err != nil {
		return nil, utils.MakeErrorTrace(ctx, err, "failed to parse assignment")
	} else if newBlocks != nil {
		block = newBlocks[len(newBlocks)-1]
	}
	rvals, blocks, err := v.genCtx.GenerateExprList(block, ctx.ExpressionList(1))
	if err != nil {
		return nil, utils.MakeErrorTrace(ctx, err, "failed to parse assignment")
	} else if blocks != nil {
		newBlocks = append(newBlocks, blocks...)
		block = newBlocks[len(newBlocks)-1]
	}
	if ctx.Assign_op().GetChildCount() != 1 {
		blocks, err := v.visitAssignOp(block, ctx.Assign_op(), lvals, rvals)
		if err != nil {
			return nil, utils.MakeErrorTrace(ctx, err, "failed to parse assignment")
		} else if blocks != nil {
			newBlocks = append(newBlocks, blocks...)
		}
		return newBlocks, nil
	}
	if len(lvals) != len(rvals) {
		return nil, utils.MakeErrorTrace(ctx, nil, "unmatched lvals(%d) and rvals(%d) count", len(lvals), len(rvals))
	}
	for i := range len(rvals) {
		if lvals[i] != nil {
			block.NewStore(rvals[i], lvals[i])
		}
	}
	return newBlocks, nil
}

func (v *CodeGenVisitor) visitAssignOp(block *ir.Block, ctx parser.IAssign_opContext, lvals, rvals []value.Value) ([]*ir.Block, error) {
	if len(lvals) != 1 || len(rvals) != 1 {
		return nil, utils.MakeErrorTrace(ctx, nil, "multiple values in sigle-valued context")
	}
	lval := block.NewLoad(lvals[0].Type().(*types.PointerType).ElemType, lvals[0])
	// check type
	ctp, ok := typesystem.CommonSupertype(lval, rvals[0])
	if !ok {
		return nil, utils.MakeErrorTrace(ctx, nil, "failed to deduce common type for values %s and %s", lvals[0].String(), rvals[0].String())
	}
	if ctx.PLUS() != nil {
		if typesystem.IsFloatType(ctp) {
			block.NewStore(block.NewFAdd(lval, rvals[0]), lvals[0])
			return nil, nil
		} else if typesystem.IsIntType(ctp) {
			block.NewStore(block.NewAdd(lval, rvals[0]), lvals[0])
			return nil, nil
		}
	} else if ctx.MINUS() != nil {
		if typesystem.IsFloatType(ctp) {
			block.NewStore(block.NewFSub(lval, rvals[0]), lvals[0])
			return nil, nil
		} else if typesystem.IsIntType(ctp) {
			block.NewStore(block.NewSub(lval, rvals[0]), lvals[0])
			return nil, nil
		}
	} else if ctx.STAR() != nil {
		if typesystem.IsFloatType(ctp) {
			block.NewStore(block.NewFMul(lval, rvals[0]), lvals[0])
			return nil, nil
		} else if typesystem.IsIntType(ctp) {
			block.NewStore(block.NewMul(lval, rvals[0]), lvals[0])
			return nil, nil
		}
	} else if ctx.DIV() != nil {
		if typesystem.IsFloatType(ctp) {
			block.NewStore(block.NewFDiv(lval, rvals[0]), lvals[0])
			return nil, nil
		} else if typesystem.IsIntType(ctp) {
			block.NewStore(block.NewSDiv(lval, rvals[0]), lvals[0])
			return nil, nil
		} else if typesystem.IsUintType(ctp) {
			block.NewStore(block.NewUDiv(lval, rvals[0]), lvals[0])
			return nil, nil
		}
	} else {
		return nil, utils.MakeErrorTrace(ctx, nil, "unsupported operator '%s'", ctx.GetText())
	}
	return nil, utils.MakeErrorTrace(ctx, nil, "unsupported type for %s operation", ctx.GetText())
}

func (v *CodeGenVisitor) VisitShortVarDecl(block *ir.Block, ctx parser.IShortVarDeclContext) ([]*ir.Block, error) {
	vals, blocks, err := v.genCtx.GenerateExprList(block, ctx.ExpressionList())
	if err != nil {
		return nil, utils.MakeErrorTrace(ctx, err, "failed to parse short var declaration")
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
		return nil, utils.MakeErrorTrace(ctx, err, "failed to parse return statement")
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
				return nil, utils.MakeErrorTrace(ctx, nil, "invalid function parameter: %s", pName)
			}
			block.NewStore(val, outPar)
		}
		block.NewRet(nil)
	}
	return newBlocks, nil
}
