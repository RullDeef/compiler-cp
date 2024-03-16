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

	loopStack []loopBlocks

	currentFuncDecl FunctionDecl
	currentFuncIR   *ir.Func
}

func NewCodeGenVisitor(pdata *PackageData) *CodeGenVisitor {
	return &CodeGenVisitor{
		packageData: pdata,
		genCtx:      NewGenContext(pdata),
	}
}

func (v *CodeGenVisitor) VisitSourceFile(ctx parser.ISourceFileContext) (*ir.Module, error) {
	// gather global declarations
	ctorFun := ir.NewFunc(fmt.Sprintf("%s_init", v.packageData.PackageName), types.Void)
	globalInitBlocks := []*ir.Block{ir.NewBlock("entry")}
	for _, decl := range ctx.AllDeclaration() {
		blocks, err := v.VisitDeclaration(globalInitBlocks[len(globalInitBlocks)-1], decl)
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
	realMainEntry.NewRet(constant.NewInt(types.I32, int64(0)))

	return module, nil
}

func (v *CodeGenVisitor) VisitDeclaration(block *ir.Block, ctx parser.IDeclarationContext) ([]*ir.Block, error) {
	var newBlocks []*ir.Block
	// populate global consts and variables
	if ctx.ConstDecl() != nil {
		for _, spec := range ctx.ConstDecl().AllConstSpec() {
			blocks, err := v.VisitConstSpec(block, spec)
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
			blocks, err := v.VisitVarSpec(block, spec)
			if err != nil {
				return nil, err
			} else if blocks != nil {
				newBlocks = append(newBlocks, blocks...)
				block = newBlocks[len(newBlocks)-1]
			}
		}
		return newBlocks, nil
	} else if ctx.TypeDecl() != nil {
		return nil, utils.MakeError("type declarations not supported yet")
	} else {
		panic("must never happen")
	}
}

func (v *CodeGenVisitor) VisitConstSpec(block *ir.Block, ctx parser.IConstSpecContext) ([]*ir.Block, error) {
	// iota and inherited declarations not supported yet
	var ids []string
	for _, id := range ctx.IdentifierList().AllIDENTIFIER() {
		ids = append(ids, id.GetText())
	}
	var vals []value.Value
	var blocks []*ir.Block
	for _, exp := range ctx.ExpressionList().AllExpression() {
		exprs, newBlocks, err := v.genCtx.GenerateExpr(block, exp)
		if err != nil {
			return nil, err
		} else if newBlocks != nil {
			blocks = append(blocks, newBlocks...)
			block = blocks[len(blocks)-1]
		}
		vals = append(vals, exprs...)
	}
	if len(ids) != len(vals) {
		return nil, utils.MakeError("umatched count of ids(%d) and vals(%d) in const spec", len(ids), len(vals))
	}
	for i := range ids {
		// global consts only
		memRef := v.genCtx.module.NewGlobal(ids[i], vals[i].Type())
		memRef.Init = &constant.ZeroInitializer{}
		memRef.Immutable = true
		block.NewStore(vals[i], memRef)
		if err := v.genCtx.Vars.Add(ids[i], memRef); err != nil {
			return nil, err
		}
	}
	return blocks, nil
}

func (v *CodeGenVisitor) VisitVarSpec(block *ir.Block, ctx parser.IVarSpecContext) ([]*ir.Block, error) {
	// iota and inherited declarations not supported yet
	var ids []string
	for _, id := range ctx.IdentifierList().AllIDENTIFIER() {
		ids = append(ids, id.GetText())
	}
	var vals []value.Value
	var blocks []*ir.Block
	if ctx.ExpressionList() != nil {
		for _, exp := range ctx.ExpressionList().AllExpression() {
			exprs, newBlocks, err := v.genCtx.GenerateExpr(block, exp)
			if err != nil {
				return nil, err
			} else if newBlocks != nil {
				blocks = append(blocks, newBlocks...)
				block = blocks[len(blocks)-1]
			}
			vals = append(vals, exprs...)
		}
	} else if ctx.Type_() != nil {
		// zero value init based on type
		if llvmType, err := goTypeToIR(ctx.Type_().GetText()); err != nil {
			return nil, err
		} else {
			for range ids {
				if itp, ok := llvmType.(*types.IntType); ok {
					vals = append(vals, constant.NewInt(itp, int64(0)))
				} else if ftp, ok := llvmType.(*types.FloatType); ok {
					vals = append(vals, constant.NewFloat(ftp, 0))
				}
			}
		}
	} else {
		// invalid situation
		return nil, utils.MakeError("invalid var spec")
	}
	if len(ids) != len(vals) {
		return nil, utils.MakeError("umatched count of ids(%d) and vals(%d) in const spec", len(ids), len(vals))
	}
	for i := range ids {
		// global consts only
		memRef := v.genCtx.module.NewGlobal(ids[i], vals[i].Type())
		memRef.Init = &constant.ZeroInitializer{}
		block.NewStore(vals[i], memRef)
		if err := v.genCtx.Vars.Add(ids[i], memRef); err != nil {
			return nil, err
		}
	}
	return blocks, nil
}

func (v *CodeGenVisitor) VisitFunctionDecl(ctx parser.IFunctionDeclContext) interface{} {
	fun, err := v.genCtx.LookupFunc(ctx.IDENTIFIER().GetText())
	if err != nil {
		return err
	}

	v.currentFuncDecl = v.packageData.Functions[fun.Name()]
	v.currentFuncIR = fun

	// setup local var storage
	v.genCtx.Vars = NewVarContext(v.genCtx.Vars)
	defer func() { v.genCtx.Vars = v.genCtx.Vars.Parent }()

	v.UID = 0

	// populate function arguments
	block := fun.NewBlock("entry")
	for _, param := range fun.Params {
		memRef := block.NewAlloca(param.Type())
		block.NewStore(param, memRef)
		v.genCtx.Vars.Add(param.Name(), memRef)
	}

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
			bodyBlocks[len(bodyBlocks)-1].NewRet(nil)
		}
		return nil
	}
}

func (v *CodeGenVisitor) VisitBlock(block *ir.Block, ctx parser.IBlockContext) ([]*ir.Block, error) {
	var blocks []*ir.Block
	v.genCtx.PushLexicalScope()
	defer v.genCtx.PopLexicalScope()
	if ctx.StatementList() != nil {
		for _, stmt := range ctx.StatementList().AllStatement() {
			switch s := stmt.GetChild(0).(type) {
			case parser.ISimpleStmtContext:
				if newBlocks, err := v.VisitSimpleStatement(block, s); err != nil {
					return nil, utils.MakeError("failed to parse assignment: %w", err)
				} else if newBlocks != nil {
					blocks = append(blocks, newBlocks...)
					block = blocks[len(blocks)-1]
				}
			case parser.IReturnStmtContext:
				if newBlocks, err := v.VisitReturnStmt(block, s); err != nil {
					return nil, utils.MakeError("invalid ret statement: %w", err)
				} else if newBlocks != nil {
					blocks = append(blocks, newBlocks...)
					block = blocks[len(blocks)-1]
				}
			case parser.IIfStmtContext:
				if newBlocks, err := v.VisitIfStmt(block, s); err != nil {
					return nil, utils.MakeError("invalid if statement: %w", err)
				} else if newBlocks != nil {
					blocks = append(blocks, newBlocks...)
					block = blocks[len(blocks)-1]
				}
			case parser.IBlockContext:
				if newBlocks, err := v.VisitBlock(block, s); err != nil {
					return nil, utils.MakeError("invalid block statement: %w", err)
				} else if newBlocks != nil {
					blocks = append(blocks, newBlocks...)
					block = blocks[len(blocks)-1]
				}
			case parser.IForStmtContext:
				if newBlocks, err := v.VisitForStmt(block, s); err != nil {
					return nil, utils.MakeError("invalid for statement: %w", err)
				} else if newBlocks != nil {
					blocks = append(blocks, newBlocks...)
					block = blocks[len(blocks)-1]
				}
			case parser.IDeclarationContext:
				if newBlocks, err := v.VisitDeclaration(block, s); err != nil {
					return nil, utils.MakeError("invalid declaration: %w", err)
				} else if newBlocks != nil {
					blocks = append(blocks, newBlocks...)
					block = blocks[len(blocks)-1]
				}
			case parser.IBreakStmtContext:
				if err := v.VisitBreakStmt(block, s); err != nil {
					return nil, err
				}
			case parser.IContinueStmtContext:
				if err := v.VisitContinueStmt(block, s); err != nil {
					return nil, err
				}
			default:
				return nil, utils.MakeError("unsupported instruction")
			}
		}
	}
	return blocks, nil
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
		res = block.NewAdd(varVal, constant.NewInt(elType.(*types.IntType), int64(1)))
	} else {
		res = block.NewSub(varVal, constant.NewInt(itype, int64(1)))
	}
	block.NewStore(res, varRef)
	return nil, nil
}

func (v *CodeGenVisitor) VisitAssignment(block *ir.Block, ctx parser.IAssignmentContext) ([]*ir.Block, error) {
	var newBlocks []*ir.Block
	for i := range ctx.ExpressionList(0).AllExpression() {
		// POTENTIALLY UNSAFE
		varName := ctx.ExpressionList(0).Expression(i).PrimaryExpr().Operand().OperandName().IDENTIFIER().GetText()
		varRef, ok := v.genCtx.Vars.Lookup(varName)
		if !ok && varName != "_" {
			return nil, utils.MakeError("var %s not defined in this context", varName)
		}
		vals, newBl, err := v.genCtx.GenerateExpr(block, ctx.ExpressionList(1).Expression(i))
		if err != nil {
			return nil, err
		}
		if newBl != nil {
			newBlocks = append(newBlocks, newBl...)
			block = newBlocks[len(newBlocks)-1]
		}
		if varName != "_" {
			block.NewStore(vals[0], varRef)
		}
	}
	return nil, nil
}

func (v *CodeGenVisitor) VisitShortVarDecl(block *ir.Block, ctx parser.IShortVarDeclContext) ([]*ir.Block, error) {
	var blocks []*ir.Block
	varIndex := 0
	for i := range ctx.ExpressionList().AllExpression() {
		vals, newBlocks, err := v.genCtx.GenerateExpr(
			block,
			ctx.ExpressionList().Expression(i),
		)
		if err != nil {
			return nil, err
		} else if newBlocks != nil {
			blocks = append(blocks, newBlocks...)
			block = blocks[len(blocks)-1]
		}
		var llvmTypes []types.Type
		var memRefs []*ir.InstAlloca
		for _, tp := range vals {
			llvmTypes = append(llvmTypes, tp.Type())
			memRefs = append(memRefs, block.NewAlloca(tp.Type()))
		}
		if structType, ok := llvmTypes[0].(*types.StructType); ok {
			for _, field := range structType.Fields {
				varName := ctx.IdentifierList().IDENTIFIER(varIndex).GetText()
				varIndex++
				if varName == "_" {
					continue
				}
				if err := v.genCtx.Vars.Add(varName, memRefs[0]); err != nil {
					return nil, err
				}
				// extract struct element
				elemRef := block.NewGetElementPtr(field, vals[0], constant.NewInt(types.I32, int64(0)), constant.NewInt(types.I32, int64(0)))
				elemVal := block.NewLoad(field, elemRef)
				block.NewStore(elemVal, memRefs[0])
			}
		} else {
			varName := ctx.IdentifierList().IDENTIFIER(varIndex).GetText()
			varIndex++
			if varName == "_" {
				continue
			}
			if err := v.genCtx.Vars.Add(varName, memRefs[0]); err != nil {
				return nil, err
			}
			block.NewStore(vals[0], memRefs[0])
		}
	}
	return blocks, nil
}

func (v *CodeGenVisitor) VisitReturnStmt(block *ir.Block, ctx parser.IReturnStmtContext) ([]*ir.Block, error) {
	// TODO: return multiple values from function
	if ctx.ExpressionList() == nil {
		block.NewRet(nil)
		return nil, nil
	}
	var newBlocks []*ir.Block
	var vals []value.Value
	for _, exprCtx := range ctx.ExpressionList().AllExpression() {
		exprs, blocks, err := v.genCtx.GenerateExpr(block, exprCtx)
		if err != nil {
			return nil, err
		} else if blocks != nil {
			newBlocks = append(newBlocks, blocks...)
			block = newBlocks[len(newBlocks)-1]
		}
		vals = append(vals, exprs...)
	}
	// match return types of function with value types
	if len(vals) == 1 {
		block.NewRet(vals[0])
	} else if len(vals) > 1 {
		retType, ok := v.currentFuncIR.Sig.RetType.(*types.StructType)
		if !ok {
			return nil, utils.MakeError("function does not return multiple values")
		}
		var fields []constant.Constant
		for _, val := range vals {
			fields = append(fields, val.(constant.Constant))
		}
		block.NewRet(constant.NewStruct(retType, fields...))
	}
	return newBlocks, nil
}
