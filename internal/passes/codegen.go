package passes

import (
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/typesystem"

	"github.com/llir/llvm/ir"
)

type CodeGenVisitor struct {
	parser.BaseGoParserVisitor
	packageData *PackageData
	genCtx      *GenContext

	// unique index per function body generation
	UID int
}

func NewCodeGenVisitor(pdata *PackageData) *CodeGenVisitor {
	return &CodeGenVisitor{
		packageData: pdata,
		genCtx:      NewGenContext(pdata),
	}
}

func (v *CodeGenVisitor) VisitSourceFile(ctx parser.ISourceFileContext) (*ir.Module, error) {
	// gather constants
	for _, decl := range ctx.AllDeclaration() {
		err := v.VisitDeclaration(decl)
		if err != nil {
			return nil, err
		}
	}

	// add code for each function declaration
	for _, fun := range ctx.AllFunctionDecl() {
		res := v.VisitFunctionDecl(fun.(*parser.FunctionDeclContext))
		if res != nil {
			return nil, fmt.Errorf("failed to parse func %s: %w", fun.IDENTIFIER().GetText(), res.(error))
		}
	}
	return v.genCtx.Module(), nil
}

func (v *CodeGenVisitor) VisitDeclaration(ctx parser.IDeclarationContext) error {
	// populate global consts and variables
	if ctx.ConstDecl() != nil {
		for _, spec := range ctx.ConstDecl().AllConstSpec() {
			err := v.VisitConstSpec(spec)
			if err != nil {
				return err
			}
		}
	}
	if ctx.VarDecl() != nil {
		for _, spec := range ctx.VarDecl().AllVarSpec() {
			err := v.VisitVarSpec(spec)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *CodeGenVisitor) VisitConstSpec(ctx parser.IConstSpecContext) error {
	// iota and inherited declarations not supported yet
	var ids []string
	for _, id := range ctx.IdentifierList().AllIDENTIFIER() {
		ids = append(ids, id.GetText())
	}
	// only constants supported right now
	tmpBlock := ir.NewBlock("")
	var vals []*typesystem.TypedValue
	for _, exp := range ctx.ExpressionList().AllExpression() {
		val, newBlocks, err := v.genCtx.GenerateExpr(tmpBlock, exp)
		_ = newBlocks
		if err != nil {
			return err
		}
		vals = append(vals, val)
	}
	if len(ids) != len(vals) {
		return fmt.Errorf("umatched count of ids(%d) and vals(%d) in const spec", len(ids), len(vals))
	}
	for i := range ids {
		v.genCtx.Vars.vars[ids[i]] = vals[i]
	}
	return nil
}

func (v *CodeGenVisitor) VisitVarSpec(ctx parser.IVarSpecContext) error {
	return fmt.Errorf("global var spec not supported rn")
}

func (v *CodeGenVisitor) VisitFunctionDecl(ctx parser.IFunctionDeclContext) interface{} {
	funName := ctx.IDENTIFIER().GetText()
	fun := v.genCtx.Funcs[funName]

	// setup local var storage
	v.genCtx.Vars = NewVarContext(v.genCtx.Vars)
	defer func() { v.genCtx.Vars = v.genCtx.Vars.Parent }()

	v.UID = 0

	// populate function arguments
	block := fun.NewBlock("entry")
	for _, param := range fun.Params {
		memRef := block.NewAlloca(param.Type())
		block.NewStore(param, memRef)
		v.genCtx.Vars.vars[param.Name()] = typesystem.NewTypedValueFromIR(
			param.Type(),
			memRef,
		)
	}

	// codegen body
	bodyBlocks, err := v.VisitBlock(block, ctx.Block())
	if err != nil {
		return fmt.Errorf("failed to parse body: %w", err)
	} else {
		for _, block := range bodyBlocks {
			block.Parent = fun
		}
		fun.Blocks = append(fun.Blocks, bodyBlocks...)
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
					return nil, fmt.Errorf("failed to parse assignment: %w", err)
				} else if newBlocks != nil {
					blocks = append(blocks, newBlocks...)
					block = blocks[len(blocks)-1]
				}
			case parser.IReturnStmtContext:
				if newBlocks, err := v.VisitReturnStmt(block, s); err != nil {
					return nil, fmt.Errorf("invalid ret statement: %w", err)
				} else if newBlocks != nil {
					blocks = append(blocks, newBlocks...)
					block = blocks[len(blocks)-1]
				}
			case parser.IIfStmtContext:
				if newBlocks, err := v.VisitIfStmt(block, s); err != nil {
					return nil, fmt.Errorf("invalid if statement: %w", err)
				} else if newBlocks != nil {
					blocks = append(blocks, newBlocks...)
					block = blocks[len(blocks)-1]
				}
			default:
				return nil, fmt.Errorf("unsupported instruction")
			}
		}
	}
	return blocks, nil
}

func (v *CodeGenVisitor) VisitIfStmt(block *ir.Block, ctx parser.IIfStmtContext) ([]*ir.Block, error) {
	if ctx.SimpleStmt() != nil {
		return nil, fmt.Errorf("unsupported init statement in if")
	}
	expr, newBlocks, err := v.genCtx.GenerateExpr(block, ctx.Expression())
	if err != nil {
		return nil, fmt.Errorf("failed to parse if expression: %w", err)
	} else if bt, ok := expr.Type.(typesystem.BasicType); !ok || bt != typesystem.Bool {
		return nil, fmt.Errorf("expression must have boolean type")
	} else if newBlocks != nil {
		block = newBlocks[len(newBlocks)-1]
	}
	stmtUID := v.UID
	v.UID++
	btrue := ir.NewBlock(fmt.Sprintf("btrue.%d", stmtUID))
	bfalse := ir.NewBlock(fmt.Sprintf("bfalse.%d", stmtUID))
	block.NewCondBr(expr.Value, btrue, bfalse)

	newBlocks = append(newBlocks, btrue)
	trueBlocks, err := v.VisitBlock(btrue, ctx.Block(0))
	if err != nil {
		return nil, err
	} else if trueBlocks != nil {
		newBlocks = append(newBlocks, trueBlocks...)
		btrue = newBlocks[len(newBlocks)-1]
	}

	// else branch TODO
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
	default:
		return nil, fmt.Errorf("unimplemented simple statement")
	}
}

func (v *CodeGenVisitor) VisitAssignment(block *ir.Block, ctx parser.IAssignmentContext) ([]*ir.Block, error) {
	var newBlocks []*ir.Block
	for i := range ctx.ExpressionList(0).AllExpression() {
		// POTENTIALLY UNSAFE
		varName := ctx.ExpressionList(0).Expression(i).PrimaryExpr().Operand().OperandName().IDENTIFIER().GetText()
		varDef, ok := v.genCtx.Vars.Lookup(varName)
		if !ok {
			return nil, fmt.Errorf("var %s not defined in this context", varName)
		}
		val, newBl, err := v.genCtx.GenerateExpr(block, ctx.ExpressionList(1).Expression(i))
		if err != nil {
			return nil, err
		}
		if newBl != nil {
			newBlocks = append(newBlocks, newBl...)
			block = newBlocks[len(newBlocks)-1]
		}
		block.NewStore(val.Value, varDef.Value)
	}
	return nil, nil
}

func (v *CodeGenVisitor) VisitShortVarDecl(block *ir.Block, ctx parser.IShortVarDeclContext) ([]*ir.Block, error) {
	varName := ctx.IdentifierList().IDENTIFIER(0).GetText()
	val, newBlocks, err := v.genCtx.GenerateExpr(
		block,
		ctx.ExpressionList().Expression(0),
	)
	if err != nil {
		return nil, err
	}
	if newBlocks != nil {
		block = newBlocks[len(newBlocks)-1]
	}
	memRef := block.NewAlloca(val.Value.Type())
	if err := v.genCtx.Vars.Add(varName, &typesystem.TypedValue{
		Type:  val.Type,
		Value: memRef,
	}); err != nil {
		return nil, err
	}
	block.NewStore(val.Value, memRef)
	return newBlocks, nil
}

func (v *CodeGenVisitor) VisitReturnStmt(block *ir.Block, ctx parser.IReturnStmtContext) ([]*ir.Block, error) {
	// TODO: return multiple values from function
	if ctx.ExpressionList() == nil {
		block.NewRet(nil)
		return nil, nil
	}
	expr1, newBlocks, err := v.genCtx.GenerateExpr(block, ctx.ExpressionList().Expression(0))
	if err != nil {
		return nil, err
	}
	if len(newBlocks) == 0 {
		block.NewRet(expr1.Value)
	} else {
		newBlocks[len(newBlocks)-1].NewRet(expr1.Value)
	}
	return newBlocks, nil
}
