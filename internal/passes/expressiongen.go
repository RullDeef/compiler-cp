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
	"github.com/llir/llvm/ir/value"
)

type TypedValue struct {
	Value value.Value
	Type  any
}

func GenerateExpr(block *ir.Block, ctx parser.IExpressionContext) (*TypedValue, error) {
	if ctx.PrimaryExpr() != nil {
		return GeneratePrimaryExpr(block, ctx.PrimaryExpr())
	} else if ctx.GetUnary_op() != nil {
		return GenerateUnaryExpr(block, ctx)
	}
	left, err := GenerateExpr(block, ctx.Expression(0))
	if err != nil {
		return nil, err
	}
	right, err := GenerateExpr(block, ctx.Expression(1))
	if err != nil {
		return nil, err
	}
	if ctx.GetMul_op() != nil {
		if ctx.STAR() != nil {
			return GenerateMulExpr(block, left, right)
		} else if ctx.DIV() != nil {
			return GenerateDivExpr(block, left, right)
		} else {
			return nil, fmt.Errorf("unimplemented instruction: %s", ctx.GetText())
		}
	} else if ctx.GetAdd_op() != nil {
		if ctx.PLUS() != nil {
			return GenerateAddExpr(block, left, right)
		} else if ctx.MINUS() != nil {
			return GenerateSubExpr(block, left, right)
		} else {
			return nil, fmt.Errorf("unimplemented instruction: %s", ctx.GetText())
		}
	} else if ctx.GetRel_op() != nil {
		return GenerateRelExpr(block, left, right, ctx)
	} else if ctx.LOGICAL_AND() != nil {
		return GenerateAndExpr(block, left, right)
	} else if ctx.LOGICAL_OR() != nil {
		return GenerateOrExpr(block, left, right)
	}
	return nil, fmt.Errorf("other types of expression not implemented")
}

func GeneratePrimaryExpr(block *ir.Block, ctx parser.IPrimaryExprContext) (*TypedValue, error) {
	if ctx.Operand() != nil {
		if ctx.Operand().Literal() != nil {
			return GenerateLiteralExpr(block, ctx.Operand().Literal())
		} else if ctx.Operand().Expression() != nil {
			return GenerateExpr(block, ctx.Operand().Expression())
		}
	}
	return nil, fmt.Errorf("unimplemented primary expression: %s", ctx.GetText())
}

func GenerateLiteralExpr(block *ir.Block, ctx parser.ILiteralContext) (*TypedValue, error) {
	if ctx.BasicLit() != nil {
		return GenerateBasicLiteralExpr(block, ctx.BasicLit())
	}
	return nil, fmt.Errorf("unimplemented basic literal: %s", ctx.GetText())
}

func GenerateBasicLiteralExpr(block *ir.Block, ctx parser.IBasicLitContext) (*TypedValue, error) {
	if ctx.NIL_LIT() != nil {
		return &TypedValue{
			Value: constant.NewNull(types.I32Ptr),
			Type:  typesystem.Nil,
		}, nil
	} else if ctx.Integer() != nil {
		intVal, err := strconv.Atoi(ctx.Integer().GetText())
		if err != nil {
			panic(fmt.Errorf("failed to convert to int: %w", err))
		}
		return &TypedValue{
			Value: constant.NewInt(types.I32, int64(intVal)),
			Type:  typesystem.Int64,
		}, nil
	}
	return nil, fmt.Errorf("not implemented basic lit: %s", ctx.GetText())
}

func GenerateUnaryExpr(block *ir.Block, ctx parser.IExpressionContext) (*TypedValue, error) {
	if ctx.PLUS() != nil {
		return GenerateExpr(block, ctx.Expression(0))
	}
	return nil, fmt.Errorf("unimplemented unary expression: %s", ctx.GetText())
}

func GenerateMulExpr(block *ir.Block, left, right *TypedValue) (*TypedValue, error) {
	if resBasicType, ok := typesystem.CommonSupertype(left.Type, right.Type); !ok {
		return nil, fmt.Errorf("failed to deduce common type for %v and %v", left.Type, right.Type)
	} else if typesystem.IsIntType(resBasicType) || typesystem.IsUintType(resBasicType) {
		return &TypedValue{
			Value: block.NewMul(left.Value, right.Value),
			Type:  resBasicType,
		}, nil
	} else if typesystem.IsFloatType(resBasicType) {
		return &TypedValue{
			Value: block.NewFMul(left.Value, right.Value),
			Type:  resBasicType,
		}, nil
	} else {
		return nil, fmt.Errorf("not implemented behavior for mul for common type %v", resBasicType)
	}
}

func GenerateDivExpr(block *ir.Block, left, right *TypedValue) (*TypedValue, error) {
	resBasicType, ok := typesystem.CommonSupertype(left.Type, right.Type)
	if !ok {
		return nil, fmt.Errorf("failed to deduce common type for %v and %v", left.Type, right.Type)
	} else if typesystem.IsIntType(resBasicType) {
		return &TypedValue{
			Value: block.NewSDiv(left.Value, right.Value),
			Type:  resBasicType,
		}, nil
	} else if typesystem.IsUintType(resBasicType) {
		return &TypedValue{
			Value: block.NewUDiv(left.Value, right.Value),
			Type:  resBasicType,
		}, nil
	} else if typesystem.IsFloatType(resBasicType) {
		return &TypedValue{
			Value: block.NewFDiv(left.Value, right.Value),
			Type:  resBasicType,
		}, nil
	} else {
		return nil, fmt.Errorf("not implemented behavior for mul for common type %v", resBasicType)
	}
}

func GenerateAddExpr(block *ir.Block, left, right *TypedValue) (*TypedValue, error) {
	resBasicType, ok := typesystem.CommonSupertype(left.Type, right.Type)
	if !ok {
		return nil, fmt.Errorf("failed to deduce common type for %v and %v", left.Type, right.Type)
	} else if typesystem.IsFloatType(resBasicType) {
		return &TypedValue{
			Value: block.NewFAdd(left.Value, right.Value),
			Type:  resBasicType,
		}, nil
	} else {
		return &TypedValue{
			Value: block.NewAdd(left.Value, right.Value),
			Type:  resBasicType,
		}, nil
	}
}

func GenerateSubExpr(block *ir.Block, left, right *TypedValue) (*TypedValue, error) {
	resBasicType, ok := typesystem.CommonSupertype(left.Type, right.Type)
	if !ok {
		return nil, fmt.Errorf("failed to deduce common type for %v and %v", left.Type, right.Type)
	} else if typesystem.IsFloatType(resBasicType) {
		return &TypedValue{
			Value: block.NewFSub(left.Value, right.Value),
			Type:  resBasicType,
		}, nil
	} else {
		return &TypedValue{
			Value: block.NewSub(left.Value, right.Value),
			Type:  resBasicType,
		}, nil
	}
}

func GenerateRelExpr(block *ir.Block, left, right *TypedValue, ctx parser.IExpressionContext) (*TypedValue, error) {
	resBasicType, ok := typesystem.CommonSupertype(left.Type, right.Type)
	if !ok {
		return nil, fmt.Errorf("failed to deduce common type for %v and %v", left.Type, right.Type)
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
			return nil, fmt.Errorf("must never happen")
		}
		return &TypedValue{
			Value: block.NewFCmp(cmpPred, left.Value, right.Value),
			Type:  typesystem.Bool,
		}, nil
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
			return nil, fmt.Errorf("must never happen")
		}
		return &TypedValue{
			Value: block.NewICmp(cmpPred, left.Value, right.Value),
			Type:  typesystem.Bool,
		}, nil
	}
}

func GenerateAndExpr(block *ir.Block, left, right *TypedValue) (*TypedValue, error) {
	bt1, ok := left.Type.(typesystem.BasicType)
	if !ok || bt1 != typesystem.Bool {
		return nil, fmt.Errorf("left value not of type bool: (got %v)", left.Type)
	}
	bt2, ok := right.Type.(typesystem.BasicType)
	if !ok || bt2 != typesystem.Bool {
		return nil, fmt.Errorf("right value not of type bool: (got %v)", left.Type)
	}
	return &TypedValue{
		Value: block.NewAnd(left.Value, right.Value),
		Type:  typesystem.Bool,
	}, nil
}

func GenerateOrExpr(block *ir.Block, left, right *TypedValue) (*TypedValue, error) {
	bt1, ok := left.Type.(typesystem.BasicType)
	if !ok || bt1 != typesystem.Bool {
		return nil, fmt.Errorf("left value not of type bool: (got %v)", left.Type)
	}
	bt2, ok := right.Type.(typesystem.BasicType)
	if !ok || bt2 != typesystem.Bool {
		return nil, fmt.Errorf("right value not of type bool: (got %v)", left.Type)
	}
	return &TypedValue{
		Value: block.NewOr(left.Value, right.Value),
		Type:  typesystem.Bool,
	}, nil
}
