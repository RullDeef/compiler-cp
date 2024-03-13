package passes

import (
	"fmt"
	"gocomp/internal/parser"

	"github.com/llir/llvm/ir"
)

type CodeGenVisitor struct {
	parser.BaseGoParserVisitor
	packageData *PackageData
	genCtx      *GenContext
}

func NewCodeGenVisitor(pdata *PackageData) *CodeGenVisitor {
	return &CodeGenVisitor{
		packageData: pdata,
		genCtx:      NewGenContext(pdata),
	}
}

func (v *CodeGenVisitor) VisitSourceFile(ctx parser.ISourceFileContext) (*ir.Module, error) {
	// add code for each function declaration
	for _, fun := range ctx.AllFunctionDecl() {
		res := v.VisitFunctionDecl(fun.(*parser.FunctionDeclContext))
		if res != nil {
			return nil, fmt.Errorf("failed to parse func %s: %w", fun.IDENTIFIER().GetText(), res.(error))
		}
	}
	return v.genCtx.Module(), nil
}

func (v *CodeGenVisitor) VisitFunctionDecl(ctx *parser.FunctionDeclContext) interface{} {
	funName := ctx.IDENTIFIER().GetText()
	fun := v.genCtx.Funcs[funName]

	// codegen body
	res := v.VisitBlock(ctx.Block().(*parser.BlockContext))
	if body, ok := res.(*ir.Block); !ok {
		return fmt.Errorf("failed to parse body: %w", res.(error))
	} else {
		body.Parent = fun
		fun.Blocks = append(fun.Blocks, body)
		return nil
	}
}

func (v *CodeGenVisitor) VisitBlock(ctx *parser.BlockContext) interface{} {
	if ctx.StatementList() == nil {
		return nil
	}
	block := ir.NewBlock("")
	for _, stmt := range ctx.StatementList().AllStatement() {
		switch s := stmt.GetChild(0).(type) {
		case parser.IReturnStmtContext:
			res := v.VisitReturnStmt(block, s)
			if stmt, ok := res.(*ir.TermRet); !ok {
				return fmt.Errorf("invalid ret statement: %w", res.(error))
			} else {
				block.Term = stmt
			}
		default:
			panic("unsupported instruction")
		}
	}
	return block
}

func (v *CodeGenVisitor) VisitReturnStmt(block *ir.Block, ctx parser.IReturnStmtContext) interface{} {
	// TODO: return multiple values from function
	if ctx.ExpressionList() == nil {
		return ir.NewRet(nil)
	}
	expr1, err := GenerateExpr(block, ctx.ExpressionList().Expression(0))
	if err != nil {
		panic(err)
	}
	return ir.NewRet(expr1.Value)
}
