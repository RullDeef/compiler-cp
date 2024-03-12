package passes

import (
	"fmt"
	"gocomp/internal/parser"
	"strconv"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type CodeGenVisitor struct {
	parser.BaseGoParserVisitor
	packageData *PackageData

	module *ir.Module
	funMap map[string]*ir.Func

	// TODO: PIE
	currFun   *ir.Func
	currBlock *ir.Block
}

func NewCodeGenVisitor(packageData *PackageData) *CodeGenVisitor {
	return &CodeGenVisitor{
		packageData: packageData,
		module:      ir.NewModule(),
		funMap:      make(map[string]*ir.Func),
	}
}

func (v *CodeGenVisitor) VisitSourceFile(ctx *parser.SourceFileContext) interface{} {
	// add code for each function declaration
	for _, fun := range ctx.AllFunctionDecl() {
		v.VisitFunctionDecl(fun.(*parser.FunctionDeclContext))
	}
	return v.module
}

func (v *CodeGenVisitor) VisitFunctionDecl(ctx *parser.FunctionDeclContext) interface{} {
	funName := ctx.IDENTIFIER().GetText()
	fun := v.packageData.Functions[funName]
	fmt.Printf("codegening func %s...\n", funName)

	var retType types.Type = types.Void
	if len(fun.ReturnTypes) >= 1 {
		var err error
		retType, err = goTypeToIR(fun.ReturnTypes[0].Type)
		if err != nil {
			panic(fmt.Errorf("unimplemented return type: %w", err))
		}
	}
	var params []*ir.Param
	for _, p := range fun.ArgTypes {
		t, err := goTypeToIR(p.Type)
		if err != nil {
			panic(fmt.Errorf("unimplemented arg type: %w", err))
		}
		params = append(params, ir.NewParam(p.Name, t))
	}
	irFun := v.module.NewFunc(funName, retType, params...)
	v.funMap[funName] = irFun
	v.currFun = irFun

	// codegen body
	v.VisitBlock(ctx.Block().(*parser.BlockContext))
	return nil
}

func (v *CodeGenVisitor) VisitBlock(ctx *parser.BlockContext) interface{} {
	if ctx.StatementList() == nil {
		return nil
	}
	v.currBlock = v.currFun.NewBlock("")
	for _, stmt := range ctx.StatementList().AllStatement() {
		switch s := stmt.GetChild(0).(type) {
		case parser.IReturnStmtContext:
			v.VisitReturnStmt(s.(*parser.ReturnStmtContext))
		default:
			panic("unsupported instruction")
		}
	}
	return nil
}

func (v *CodeGenVisitor) VisitReturnStmt(ctx *parser.ReturnStmtContext) interface{} {
	// TODO: return multiple values from function
	var exprs []value.Value // local variable names
	if ctx.ExpressionList() == nil {
		v.currBlock.NewRet(nil)
	} else {
		exprs = append(exprs, v.VisitExpression(ctx.ExpressionList().Expression(0).(*parser.ExpressionContext)).(value.Value))
		v.currBlock.NewRet(exprs[0])
	}
	return nil
}

func (v *CodeGenVisitor) VisitExpression(ctx *parser.ExpressionContext) interface{} {
	if ctx.PrimaryExpr() != nil {
		return v.VisitPrimaryExpr(ctx.PrimaryExpr().(*parser.PrimaryExprContext))
	} else if ctx.GetUnary_op() != nil {
		return v.VisitUnaryExpr(ctx)
	} else {
		left := v.VisitExpression(ctx.Expression(0).(*parser.ExpressionContext)).(value.Value)
		right := v.VisitExpression(ctx.Expression(1).(*parser.ExpressionContext)).(value.Value)
		if ctx.GetMul_op() != nil {
			if ctx.STAR() != nil {
				return v.currBlock.NewMul(left, right)
			} else if ctx.DIV() != nil {
				return v.currBlock.NewSDiv(left, right)
			} else {
				panic(fmt.Errorf("unimplemented instruction: %s", ctx.GetText()))
			}
		} else if ctx.GetAdd_op() != nil {
			if ctx.PLUS() != nil {
				return v.currBlock.NewAdd(left, right)
			} else if ctx.MINUS() != nil {
				return v.currBlock.NewSub(left, right)
			} else {
				panic(fmt.Errorf("unimplemented instruction: %s", ctx.GetText()))
			}
		} else if ctx.GetRel_op() != nil {
			if ctx.EQUALS() != nil {
				return v.currBlock.NewICmp(enum.IPredEQ, left, right)
			} else if ctx.NOT_EQUALS() != nil {
				return v.currBlock.NewICmp(enum.IPredNE, left, right)
			} else if ctx.LESS() != nil {
				return v.currBlock.NewICmp(enum.IPredSLT, left, right)
			} else if ctx.LESS_OR_EQUALS() != nil {
				return v.currBlock.NewICmp(enum.IPredSLE, left, right)
			} else if ctx.GREATER() != nil {
				return v.currBlock.NewICmp(enum.IPredSGT, left, right)
			} else if ctx.GREATER_OR_EQUALS() != nil {
				return v.currBlock.NewICmp(enum.IPredSGE, left, right)
			} else {
				panic("must never happen")
			}
		} else if ctx.LOGICAL_AND() != nil {
			v.currBlock.NewAnd(left, right)
		} else if ctx.LOGICAL_OR() != nil {
			v.currBlock.NewOr(left, right)
		} else {
			panic("must never happen")
		}
	}
	panic("other types of expression not implemented")
}

func (v *CodeGenVisitor) VisitPrimaryExpr(ctx *parser.PrimaryExprContext) interface{} {
	if ctx.Operand() != nil {
		return v.VisitOperand(ctx.Operand().(*parser.OperandContext))
	}
	panic("other types of primary expression not implemented")
}

func (v *CodeGenVisitor) VisitOperand(ctx *parser.OperandContext) interface{} {
	if ctx.Literal() != nil {
		return v.VisitLiteral(ctx.Literal().(*parser.LiteralContext))
	} else if ctx.OperandName() != nil {
		// TODO: lookup name
		panic("todo: lookup name")
	} else if ctx.Expression() != nil {
		return v.VisitExpression(ctx.Expression().(*parser.ExpressionContext))
	} else {
		panic("impossible situation")
	}
}

func (v *CodeGenVisitor) VisitLiteral(ctx *parser.LiteralContext) interface{} {
	if ctx.BasicLit() != nil {
		return v.VisitBasicLit(ctx.BasicLit().(*parser.BasicLitContext))
	}
	panic("composite and func literals not implemented")
}

func (v *CodeGenVisitor) VisitBasicLit(ctx *parser.BasicLitContext) interface{} {
	if ctx.NIL_LIT() != nil {
		return constant.NewNull(types.I32Ptr)
	} else if ctx.Integer() != nil {
		intVal, err := strconv.Atoi(ctx.Integer().GetText())
		if err != nil {
			panic(fmt.Errorf("failed to convert to int: %w", err))
		}
		return constant.NewInt(types.I32, int64(intVal))
	}

	panic("not implemented basic lit")
}

func (v *CodeGenVisitor) VisitUnaryExpr(ctx *parser.ExpressionContext) value.Value {
	panic("unary expression not implemented")
}

func goTypeToIR(goType string) (types.Type, error) {
	t, ok := map[string]types.Type{
		"":     types.Void,
		"int":  types.I32,
		"uint": types.I32,
	}[goType]
	if !ok {
		return nil, fmt.Errorf("invalid primitive type: %s", goType)
	}
	return t, nil
}
