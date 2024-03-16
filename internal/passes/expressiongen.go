package passes

import (
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/typesystem"
	"gocomp/internal/utils"
	"strconv"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

func (genCtx *GenContext) GenerateExpr(block *ir.Block, ctx parser.IExpressionContext) ([]value.Value, []*ir.Block, error) {
	if ctx.PrimaryExpr() != nil {
		return genCtx.GeneratePrimaryExpr(block, ctx.PrimaryExpr())
	} else if ctx.GetUnary_op() != nil {
		return genCtx.GenerateUnaryExpr(block, ctx)
	}
	//TODO: lazy logical expression evaluation
	left, leftBlocks, err := genCtx.GenerateExpr(block, ctx.Expression(0))
	_ = leftBlocks
	if err != nil {
		return nil, nil, err
	}
	right, rightBlocks, err := genCtx.GenerateExpr(block, ctx.Expression(1))
	_ = rightBlocks
	if err != nil {
		return nil, nil, err
	}
	if ctx.LOGICAL_AND() != nil {
		return genCtx.GenerateAndExpr(block, left[0], right[0])
	} else if ctx.LOGICAL_OR() != nil {
		return genCtx.GenerateOrExpr(block, left[0], right[0])
	}
	if ctx.GetMul_op() != nil {
		if ctx.STAR() != nil {
			return genCtx.GenerateMulExpr(block, left[0], right[0])
		} else if ctx.DIV() != nil {
			return genCtx.GenerateDivExpr(block, left[0], right[0])
		} else if ctx.MOD() != nil {
			return genCtx.GenerateModExpr(block, left[0], right[0])
		} else {
			return nil, nil, utils.MakeError("unimplemented instruction: %s", ctx.GetText())
		}
	} else if ctx.GetAdd_op() != nil {
		if ctx.PLUS() != nil {
			return genCtx.GenerateAddExpr(block, left[0], right[0])
		} else if ctx.MINUS() != nil {
			return genCtx.GenerateSubExpr(block, left[0], right[0])
		} else {
			return nil, nil, utils.MakeError("unimplemented instruction: %s", ctx.GetText())
		}
	} else if ctx.GetRel_op() != nil {
		return genCtx.GenerateRelExpr(block, left[0], right[0], ctx)
	}

	return nil, nil, utils.MakeError("other types of expression not implemented")
}

func (genCtx *GenContext) GeneratePrimaryExpr(block *ir.Block, ctx parser.IPrimaryExprContext) ([]value.Value, []*ir.Block, error) {
	if ctx.Operand() != nil {
		return genCtx.GenerateOperand(block, ctx.Operand())
	} else if ctx.PrimaryExpr() != nil && ctx.Arguments() != nil {
		// function call
		funName := ctx.PrimaryExpr().Operand().OperandName().IDENTIFIER().GetText()
		funRef, err := genCtx.LookupFunc(funName)
		if err != nil {
			return nil, nil, err
		}
		args, blocks, err := genCtx.GenerateArguments(block, ctx.Arguments())
		if err != nil {
			return nil, nil, err
		} else if blocks != nil {
			block = blocks[len(blocks)-1]
		}
		res := block.NewCall(funRef, args...)
		return []value.Value{
			typesystem.NewTypedValue(
				res,
				funRef.Sig.RetType,
			),
		}, blocks, nil
	}
	return nil, nil, utils.MakeError("unimplemented primary expression: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateArguments(block *ir.Block, ctx parser.IArgumentsContext) ([]value.Value, []*ir.Block, error) {
	if ctx.ExpressionList() == nil {
		return nil, nil, nil
	}
	var vals []value.Value
	var blocks []*ir.Block
	for _, expr := range ctx.ExpressionList().AllExpression() {
		tval, newBlocks, err := genCtx.GenerateExpr(block, expr)
		if err != nil {
			return nil, nil, err
		} else if newBlocks != nil {
			blocks = append(blocks, newBlocks...)
			block = blocks[len(blocks)-1]
		}
		vals = append(vals, tval...)
	}
	return vals, blocks, nil
}

func (genCtx *GenContext) GenerateOperand(block *ir.Block, ctx parser.IOperandContext) ([]value.Value, []*ir.Block, error) {
	if ctx.Literal() != nil {
		return genCtx.GenerateLiteralExpr(block, ctx.Literal())
	} else if ctx.OperandName() != nil {
		// lookup value
		varName := ctx.OperandName().IDENTIFIER().GetText()
		if val, ok := genCtx.Vars.Lookup(varName); !ok {
			return nil, nil, utils.MakeError("variable %s not defined in this scope", varName)
		} else {
			ld := block.NewLoad(val.Type().(*types.PointerType).ElemType, val)
			return []value.Value{
				typesystem.NewTypedValue(
					ld,
					val.Type().(*types.PointerType).ElemType,
				),
			}, nil, nil
		}
	} else if ctx.Expression() != nil {
		return genCtx.GenerateExpr(block, ctx.Expression())
	}
	return nil, nil, utils.MakeError("unmplemented operand")
}

func (genCtx *GenContext) GenerateLiteralExpr(block *ir.Block, ctx parser.ILiteralContext) ([]value.Value, []*ir.Block, error) {
	if ctx.BasicLit() != nil {
		return genCtx.GenerateBasicLiteralExpr(block, ctx.BasicLit())
	}
	return nil, nil, utils.MakeError("unimplemented basic literal: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateBasicLiteralExpr(block *ir.Block, ctx parser.IBasicLitContext) ([]value.Value, []*ir.Block, error) {
	if ctx.NIL_LIT() != nil {
		return []value.Value{constant.NewNull(types.I32Ptr)}, nil, nil
	} else if ctx.Integer() != nil {
		val, err := constant.NewIntFromString(types.I32, ctx.Integer().GetText())
		if err != nil {
			return nil, nil, err
		}
		return []value.Value{val}, nil, nil
	} else if ctx.FLOAT_LIT() != nil {
		val, err := constant.NewFloatFromString(types.Double, ctx.FLOAT_LIT().GetText())
		if err != nil {
			return nil, nil, err
		}
		return []value.Value{
			typesystem.NewTypedValue(val, typesystem.Float64),
		}, nil, nil
	} else if ctx.FALSE_LIT() != nil {
		return []value.Value{
			typesystem.NewTypedValue(constant.False, typesystem.Bool),
		}, nil, nil
	} else if ctx.TRUE_LIT() != nil {
		return []value.Value{
			typesystem.NewTypedValue(constant.True, typesystem.Bool),
		}, nil, nil
	} else if ctx.String_() != nil {
		strVal, err := strconv.Unquote(ctx.String_().GetText())
		if err != nil {
			return nil, nil, err
		}
		var glob *ir.Global
		var ok bool
		if glob, ok = genCtx.Consts[strVal]; !ok {
			val := constant.NewCharArray(append([]byte(strVal), byte(0)))
			glob = genCtx.module.NewGlobalDef(fmt.Sprintf("str.%d", len(genCtx.Consts)), val)
			genCtx.Consts[strVal] = glob
		}
		addr := constant.NewGetElementPtr(glob.ContentType, glob, constant.NewInt(types.I32, 0), constant.NewInt(types.I32, 0))
		return []value.Value{
			typesystem.NewTypedValue(addr, glob.Type()),
		}, nil, nil
	}
	return nil, nil, utils.MakeError("not implemented basic lit: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateUnaryExpr(block *ir.Block, ctx parser.IExpressionContext) ([]value.Value, []*ir.Block, error) {
	if ctx.PLUS() != nil {
		return genCtx.GenerateExpr(block, ctx.Expression(0))
	} else if ctx.EXCLAMATION() != nil {
		vals, blocks, err := genCtx.GenerateExpr(block, ctx.Expression(0))
		if err != nil {
			return nil, nil, err
		} else if blocks != nil {
			block = blocks[len(blocks)-1]
		}
		return []value.Value{
			typesystem.NewTypedValue(
				block.NewXor(vals[0], constant.True),
				typesystem.Bool,
			),
		}, blocks, nil
	}
	return nil, nil, utils.MakeError("unimplemented unary expression: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateMulExpr(block *ir.Block, left, right value.Value) ([]value.Value, []*ir.Block, error) {
	if resType, ok := typesystem.CommonSupertype(left.Type(), right.Type()); !ok {
		return nil, nil, utils.MakeError("failed to deduce common type for %v and %v", left.Type(), right.Type())
	} else if typesystem.IsIntType(resType) || typesystem.IsUintType(resType) {
		return []value.Value{
			typesystem.NewTypedValue(block.NewMul(left, right), resType),
		}, nil, nil
	} else if typesystem.IsFloatType(resType) {
		return []value.Value{
			typesystem.NewTypedValue(block.NewFMul(left, right), resType),
		}, nil, nil
	} else {
		return nil, nil, utils.MakeError("not implemented mul for type %+v", resType)
	}
}

func (genCtx *GenContext) GenerateDivExpr(block *ir.Block, left, right value.Value) ([]value.Value, []*ir.Block, error) {
	if resType, ok := typesystem.CommonSupertype(left.Type(), right.Type()); !ok {
		return nil, nil, utils.MakeError("failed to deduce common type for %v and %v", left.Type(), right.Type())
	} else if typesystem.IsIntType(resType) {
		return []value.Value{
			typesystem.NewTypedValue(block.NewSDiv(left, right), resType),
		}, nil, nil
	} else if typesystem.IsUintType(resType) {
		return []value.Value{
			typesystem.NewTypedValue(block.NewUDiv(left, right), resType),
		}, nil, nil
	} else if typesystem.IsFloatType(resType) {
		return []value.Value{
			typesystem.NewTypedValue(block.NewFDiv(left, right), resType),
		}, nil, nil
	} else {
		return nil, nil, utils.MakeError("not implemented div for type %+v", resType)
	}
}

func (genCtx *GenContext) GenerateModExpr(block *ir.Block, left, right value.Value) ([]value.Value, []*ir.Block, error) {
	if resType, ok := typesystem.CommonSupertype(left.Type(), right.Type()); !ok {
		return nil, nil, utils.MakeError("failed to deduce common type for %v and %v", left.Type(), right.Type())
	} else if typesystem.IsIntType(resType) {
		return []value.Value{
			typesystem.NewTypedValue(block.NewSRem(left, right), resType),
		}, nil, nil
	} else if typesystem.IsUintType(resType) {
		return []value.Value{
			typesystem.NewTypedValue(block.NewURem(left, right), resType),
		}, nil, nil
	} else {
		return nil, nil, utils.MakeError("not implemented behavior for mod")
	}
}

func (genCtx *GenContext) GenerateAddExpr(block *ir.Block, left, right value.Value) ([]value.Value, []*ir.Block, error) {
	if resType, ok := typesystem.CommonSupertype(left.Type(), right.Type()); !ok {
		return nil, nil, utils.MakeError("failed to deduce common type for %v and %v", left.Type(), right.Type())
	} else if typesystem.IsIntType(resType) || typesystem.IsUintType(resType) {
		return []value.Value{
			typesystem.NewTypedValue(block.NewAdd(left, right), resType),
		}, nil, nil
	} else if typesystem.IsFloatType(resType) {
		return []value.Value{
			typesystem.NewTypedValue(block.NewFAdd(left, right), resType),
		}, nil, nil
	} else {
		return nil, nil, utils.MakeError("not implemented add for type %+v", resType)
	}
}

func (genCtx *GenContext) GenerateSubExpr(block *ir.Block, left, right value.Value) ([]value.Value, []*ir.Block, error) {
	if resType, ok := typesystem.CommonSupertype(left.Type(), right.Type()); !ok {
		return nil, nil, utils.MakeError("failed to deduce common type for %v and %v", left.Type(), right.Type())
	} else if typesystem.IsIntType(resType) || typesystem.IsUintType(resType) {
		return []value.Value{
			typesystem.NewTypedValue(block.NewSub(left, right), resType),
		}, nil, nil
	} else if typesystem.IsFloatType(resType) {
		return []value.Value{
			typesystem.NewTypedValue(block.NewFSub(left, right), resType),
		}, nil, nil
	} else {
		return nil, nil, utils.MakeError("not implemented sub for type %+v", resType)
	}
}

func (genCtx *GenContext) GenerateRelExpr(block *ir.Block, left, right value.Value, ctx parser.IExpressionContext) ([]value.Value, []*ir.Block, error) {
	resType, ok := typesystem.CommonSupertype(left.Type(), right.Type())
	if !ok {
		return nil, nil, utils.MakeError("failed to deduce common type for %v and %v", left.Type(), right.Type())
	}
	if _, ok := resType.(*types.FloatType); ok {
		var cmpPred enum.FPred
		if ctx.EQUALS() != nil {
			cmpPred = enum.FPredOEQ
		} else if ctx.NOT_EQUALS() != nil {
			cmpPred = enum.FPredONE
		} else if ctx.LESS() != nil {
			cmpPred = enum.FPredOLT
		} else if ctx.LESS_OR_EQUALS() != nil {
			cmpPred = enum.FPredOLE
		} else if ctx.GREATER() != nil {
			cmpPred = enum.FPredOGT
		} else if ctx.GREATER_OR_EQUALS() != nil {
			cmpPred = enum.FPredOGE
		} else {
			return nil, nil, utils.MakeError("must never happen")
		}
		return []value.Value{
			typesystem.NewTypedValue(
				block.NewFCmp(cmpPred, left, right),
				typesystem.Bool,
			),
		}, nil, nil
	} else {
		var cmpPred enum.IPred
		if ctx.EQUALS() != nil {
			cmpPred = enum.IPredEQ
		} else if ctx.NOT_EQUALS() != nil {
			cmpPred = enum.IPredNE
		} else if ctx.LESS() != nil {
			if _, ok := resType.(*types.IntType); ok {
				cmpPred = enum.IPredULT
			} else {
				cmpPred = enum.IPredSLT
			}
		} else if ctx.LESS_OR_EQUALS() != nil {
			if _, ok := resType.(*types.IntType); ok {
				cmpPred = enum.IPredULE
			} else {
				cmpPred = enum.IPredSLE
			}
		} else if ctx.GREATER() != nil {
			if _, ok := resType.(*types.IntType); ok {
				cmpPred = enum.IPredUGT
			} else {
				cmpPred = enum.IPredSGT
			}
		} else if ctx.GREATER_OR_EQUALS() != nil {
			if _, ok := resType.(*types.IntType); ok {
				cmpPred = enum.IPredUGE
			} else {
				cmpPred = enum.IPredSGE
			}
		} else {
			return nil, nil, utils.MakeError("must never happen")
		}
		return []value.Value{
			typesystem.NewTypedValue(
				block.NewICmp(cmpPred, left, right),
				typesystem.Bool,
			),
		}, nil, nil
	}
}

func (genCtx *GenContext) GenerateAndExpr(block *ir.Block, left, right value.Value) ([]value.Value, []*ir.Block, error) {
	if !left.Type().Equal(typesystem.Bool) {
		return nil, nil, utils.MakeError("left value not of type bool: (got %v)", left.Type)
	}
	if !right.Type().Equal(typesystem.Bool) {
		return nil, nil, utils.MakeError("right value not of type bool: (got %v)", left.Type)
	}
	return []value.Value{
		typesystem.NewTypedValue(block.NewAnd(left, right), typesystem.Bool),
	}, nil, nil
}

func (genCtx *GenContext) GenerateOrExpr(block *ir.Block, left, right value.Value) ([]value.Value, []*ir.Block, error) {
	if !left.Type().Equal(typesystem.Bool) {
		return nil, nil, utils.MakeError("left value not of type bool: (got %v)", left.Type)
	}
	if !right.Type().Equal(typesystem.Bool) {
		return nil, nil, utils.MakeError("right value not of type bool: (got %v)", left.Type)
	}
	return []value.Value{
		typesystem.NewTypedValue(block.NewOr(left, right), typesystem.Bool),
	}, nil, nil
}
