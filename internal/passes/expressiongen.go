package passes

import (
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/typesystem"
	"strconv"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
)

func (genCtx *GenContext) GenerateExpr(block *ir.Block, ctx parser.IExpressionContext) (*typesystem.TypedValue, []*ir.Block, error) {
	if ctx.PrimaryExpr() != nil {
		return genCtx.GeneratePrimaryExpr(block, ctx.PrimaryExpr())
	} else if ctx.GetUnary_op() != nil {
		return genCtx.GenerateUnaryExpr(block, ctx)
	}
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
		return genCtx.GenerateAndExpr(block, left, right)
	} else if ctx.LOGICAL_OR() != nil {
		return genCtx.GenerateOrExpr(block, left, right)
	}
	if ctx.GetMul_op() != nil {
		if ctx.STAR() != nil {
			return genCtx.GenerateMulExpr(block, left, right)
		} else if ctx.DIV() != nil {
			return genCtx.GenerateDivExpr(block, left, right)
		} else {
			return nil, nil, fmt.Errorf("unimplemented instruction: %s", ctx.GetText())
		}
	} else if ctx.GetAdd_op() != nil {
		if ctx.PLUS() != nil {
			return genCtx.GenerateAddExpr(block, left, right)
		} else if ctx.MINUS() != nil {
			return genCtx.GenerateSubExpr(block, left, right)
		} else {
			return nil, nil, fmt.Errorf("unimplemented instruction: %s", ctx.GetText())
		}
	} else if ctx.GetRel_op() != nil {
		return genCtx.GenerateRelExpr(block, left, right, ctx)
	}

	return nil, nil, fmt.Errorf("other types of expression not implemented")
}

func (genCtx *GenContext) GeneratePrimaryExpr(block *ir.Block, ctx parser.IPrimaryExprContext) (*typesystem.TypedValue, []*ir.Block, error) {
	if ctx.Operand() != nil {
		return genCtx.GenerateOperand(block, ctx.Operand())
	}
	return nil, nil, fmt.Errorf("unimplemented primary expression: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateOperand(block *ir.Block, ctx parser.IOperandContext) (*typesystem.TypedValue, []*ir.Block, error) {
	if ctx.Literal() != nil {
		return genCtx.GenerateLiteralExpr(block, ctx.Literal())
	} else if ctx.OperandName() != nil {
		// lookup value
		varName := ctx.OperandName().IDENTIFIER().GetText()
		if val, ok := genCtx.Vars.Lookup(varName); !ok {
			return nil, nil, fmt.Errorf("variable %s not defined in this scope", varName)
		} else {
			llvmType, err := val.LLVMType()
			if err != nil {
				return nil, nil, err
			}
			ld := block.NewLoad(llvmType, val.Value)
			return &typesystem.TypedValue{
				Value: ld,
				Type:  val.Type,
			}, nil, nil
		}
	} else if ctx.Expression() != nil {
		return genCtx.GenerateExpr(block, ctx.Expression())
	}
	return nil, nil, fmt.Errorf("unmplemented operand")
}

func (genCtx *GenContext) GenerateLiteralExpr(block *ir.Block, ctx parser.ILiteralContext) (*typesystem.TypedValue, []*ir.Block, error) {
	if ctx.BasicLit() != nil {
		return genCtx.GenerateBasicLiteralExpr(block, ctx.BasicLit())
	}
	return nil, nil, fmt.Errorf("unimplemented basic literal: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateBasicLiteralExpr(block *ir.Block, ctx parser.IBasicLitContext) (*typesystem.TypedValue, []*ir.Block, error) {
	if ctx.NIL_LIT() != nil {
		return &typesystem.TypedValue{
			Value: constant.NewNull(types.I32Ptr),
			Type:  typesystem.Nil,
		}, nil, nil
	} else if ctx.Integer() != nil {
		intVal, err := strconv.Atoi(ctx.Integer().GetText())
		if err != nil {
			panic(fmt.Errorf("failed to convert to int: %w", err))
		}
		return &typesystem.TypedValue{
			Value: constant.NewInt(types.I32, int64(intVal)),
			Type:  typesystem.Int64,
		}, nil, nil
	} else if ctx.FLOAT_LIT() != nil {
		fltVal, err := strconv.ParseFloat(ctx.FLOAT_LIT().GetText(), 64)
		if err != nil {
			panic(fmt.Errorf("failed to convert to float64: %w", err))
		}
		return &typesystem.TypedValue{
			Value: constant.NewFloat(types.Double, fltVal),
			Type:  typesystem.Float64,
		}, nil, nil
	}
	return nil, nil, fmt.Errorf("not implemented basic lit: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateUnaryExpr(block *ir.Block, ctx parser.IExpressionContext) (*typesystem.TypedValue, []*ir.Block, error) {
	if ctx.PLUS() != nil {
		return genCtx.GenerateExpr(block, ctx.Expression(0))
	}
	return nil, nil, fmt.Errorf("unimplemented unary expression: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateMulExpr(block *ir.Block, left, right *typesystem.TypedValue) (*typesystem.TypedValue, []*ir.Block, error) {
	if resBasicType, ok := typesystem.CommonSupertype(left.Type, right.Type); !ok {
		return nil, nil, fmt.Errorf("failed to deduce common type for %v and %v", left.Type, right.Type)
	} else if typesystem.IsIntType(resBasicType) || typesystem.IsUintType(resBasicType) {
		return &typesystem.TypedValue{
			Value: block.NewMul(left.Value, right.Value),
			Type:  resBasicType,
		}, nil, nil
	} else if typesystem.IsFloatType(resBasicType) {
		return &typesystem.TypedValue{
			Value: block.NewFMul(left.Value, right.Value),
			Type:  resBasicType,
		}, nil, nil
	} else {
		return nil, nil, fmt.Errorf("not implemented behavior for mul for common type %v", resBasicType)
	}
}

func (genCtx *GenContext) GenerateDivExpr(block *ir.Block, left, right *typesystem.TypedValue) (*typesystem.TypedValue, []*ir.Block, error) {
	resBasicType, ok := typesystem.CommonSupertype(left.Type, right.Type)
	if !ok {
		return nil, nil, fmt.Errorf("failed to deduce common type for %v and %v", left.Type, right.Type)
	} else if typesystem.IsIntType(resBasicType) {
		return &typesystem.TypedValue{
			Value: block.NewSDiv(left.Value, right.Value),
			Type:  resBasicType,
		}, nil, nil
	} else if typesystem.IsUintType(resBasicType) {
		return &typesystem.TypedValue{
			Value: block.NewUDiv(left.Value, right.Value),
			Type:  resBasicType,
		}, nil, nil
	} else if typesystem.IsFloatType(resBasicType) {
		return &typesystem.TypedValue{
			Value: block.NewFDiv(left.Value, right.Value),
			Type:  resBasicType,
		}, nil, nil
	} else {
		return nil, nil, fmt.Errorf("not implemented behavior for mul for common type %v", resBasicType)
	}
}

func (genCtx *GenContext) GenerateAddExpr(block *ir.Block, left, right *typesystem.TypedValue) (*typesystem.TypedValue, []*ir.Block, error) {
	resBasicType, ok := typesystem.CommonSupertype(left.Type, right.Type)
	if !ok {
		return nil, nil, fmt.Errorf("failed to deduce common type for %v and %v", left.Type, right.Type)
	} else if typesystem.IsFloatType(resBasicType) {
		return &typesystem.TypedValue{
			Value: block.NewFAdd(left.Value, right.Value),
			Type:  resBasicType,
		}, nil, nil
	} else {
		return &typesystem.TypedValue{
			Value: block.NewAdd(left.Value, right.Value),
			Type:  resBasicType,
		}, nil, nil
	}
}

func (genCtx *GenContext) GenerateSubExpr(block *ir.Block, left, right *typesystem.TypedValue) (*typesystem.TypedValue, []*ir.Block, error) {
	resBasicType, ok := typesystem.CommonSupertype(left.Type, right.Type)
	if !ok {
		return nil, nil, fmt.Errorf("failed to deduce common type for %v and %v", left.Type, right.Type)
	} else if typesystem.IsFloatType(resBasicType) {
		return &typesystem.TypedValue{
			Value: block.NewFSub(left.Value, right.Value),
			Type:  resBasicType,
		}, nil, nil
	} else {
		return &typesystem.TypedValue{
			Value: block.NewSub(left.Value, right.Value),
			Type:  resBasicType,
		}, nil, nil
	}
}

func (genCtx *GenContext) GenerateRelExpr(block *ir.Block, left, right *typesystem.TypedValue, ctx parser.IExpressionContext) (*typesystem.TypedValue, []*ir.Block, error) {
	resBasicType, ok := typesystem.CommonSupertype(left.Type, right.Type)
	if !ok {
		return nil, nil, fmt.Errorf("failed to deduce common type for %v and %v", left.Type, right.Type)
	}
	if typesystem.IsFloatType(resBasicType) {
		var cmpPred enum.FPred
		if ctx.EQUALS() != nil {
			cmpPred = enum.FPredOEQ
		} else if ctx.NOT_EQUALS() != nil {
			cmpPred = enum.FPredONE
		} else if ctx.LESS() != nil {
			cmpPred = enum.FPredOLE
		} else if ctx.LESS_OR_EQUALS() != nil {
			cmpPred = enum.FPredOLE
		} else if ctx.GREATER() != nil {
			cmpPred = enum.FPredOGT
		} else if ctx.GREATER_OR_EQUALS() != nil {
			cmpPred = enum.FPredOGE
		} else {
			return nil, nil, fmt.Errorf("must never happen")
		}
		return &typesystem.TypedValue{
			Value: block.NewFCmp(cmpPred, left.Value, right.Value),
			Type:  typesystem.Bool,
		}, nil, nil
	} else {
		var cmpPred enum.IPred
		if ctx.EQUALS() != nil {
			cmpPred = enum.IPredEQ
		} else if ctx.NOT_EQUALS() != nil {
			cmpPred = enum.IPredNE
		} else if ctx.LESS() != nil {
			if typesystem.IsUintType(resBasicType) {
				cmpPred = enum.IPredULE
			} else {
				cmpPred = enum.IPredSLT
			}
		} else if ctx.LESS_OR_EQUALS() != nil {
			if typesystem.IsUintType(resBasicType) {
				cmpPred = enum.IPredULE
			} else {
				cmpPred = enum.IPredSLE
			}
		} else if ctx.GREATER() != nil {
			if typesystem.IsUintType(resBasicType) {
				cmpPred = enum.IPredUGT
			} else {
				cmpPred = enum.IPredSGT
			}
		} else if ctx.GREATER_OR_EQUALS() != nil {
			if typesystem.IsUintType(resBasicType) {
				cmpPred = enum.IPredUGE
			} else {
				cmpPred = enum.IPredSGE
			}
		} else {
			return nil, nil, fmt.Errorf("must never happen")
		}
		return &typesystem.TypedValue{
			Value: block.NewICmp(cmpPred, left.Value, right.Value),
			Type:  typesystem.Bool,
		}, nil, nil
	}
}

func (genCtx *GenContext) GenerateAndExpr(block *ir.Block, left, right *typesystem.TypedValue) (*typesystem.TypedValue, []*ir.Block, error) {
	bt1, ok := left.Type.(typesystem.BasicType)
	if !ok || bt1 != typesystem.Bool {
		return nil, nil, fmt.Errorf("left value not of type bool: (got %v)", left.Type)
	}
	bt2, ok := right.Type.(typesystem.BasicType)
	if !ok || bt2 != typesystem.Bool {
		return nil, nil, fmt.Errorf("right value not of type bool: (got %v)", left.Type)
	}
	return &typesystem.TypedValue{
		Value: block.NewAnd(left.Value, right.Value),
		Type:  typesystem.Bool,
	}, nil, nil
}

func (genCtx *GenContext) GenerateOrExpr(block *ir.Block, left, right *typesystem.TypedValue) (*typesystem.TypedValue, []*ir.Block, error) {
	bt1, ok := left.Type.(typesystem.BasicType)
	if !ok || bt1 != typesystem.Bool {
		return nil, nil, fmt.Errorf("left value not of type bool: (got %v)", left.Type)
	}
	bt2, ok := right.Type.(typesystem.BasicType)
	if !ok || bt2 != typesystem.Bool {
		return nil, nil, fmt.Errorf("right value not of type bool: (got %v)", left.Type)
	}
	return &typesystem.TypedValue{
		Value: block.NewOr(left.Value, right.Value),
		Type:  typesystem.Bool,
	}, nil, nil
}
