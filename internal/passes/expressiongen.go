package passes

import (
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/typesystem"
	"gocomp/internal/utils"
	"strconv"

	"github.com/antlr4-go/antlr/v4"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

func (genCtx *GenContext) GenerateLValue(block *ir.Block, ctx parser.IExpressionContext) ([]value.Value, []*ir.Block, error) {
	if ctx.AMPERSAND() != nil {
		return genCtx.GenerateLValue(block, ctx.Expression(0))
	} else if ctx.STAR() != nil {
		vals, blocks, err := genCtx.GenerateExpr(block, ctx.Expression(0))
		if err != nil {
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse lvalue")
		} else if blocks != nil {
			block = blocks[len(blocks)-1]
		}
		ptrtp, ok := vals[0].Type().(*types.PointerType)
		if !ok {
			return nil, nil, utils.MakeErrorTrace(ctx, nil, "invalid lvalue type")
		}
		return []value.Value{
			typesystem.NewTypedValue(vals[0], ptrtp),
		}, blocks, nil
	} else if ctx.PrimaryExpr() != nil && ctx.PrimaryExpr().Index() != nil {
		// array indexing
		subexprs, blocks, err := genCtx.GeneratePrimaryLValue(block, ctx.PrimaryExpr().PrimaryExpr())
		if err != nil {
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse array indexing")
		} else if blocks != nil {
			block = blocks[len(blocks)-1]
		}
		idxs, newBlocks, err := genCtx.GenerateExpr(block, ctx.PrimaryExpr().Index().Expression())
		if err != nil {
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse array indexing")
		} else if newBlocks != nil {
			blocks = append(blocks, newBlocks...)
			block = blocks[len(blocks)-1]
		}
		tp := subexprs[0].Type().(*types.PointerType).ElemType
		arrtp, ok := tp.(*types.ArrayType)
		if !ok {
			ptp, ok := tp.(*types.PointerType)
			if !ok {
				return nil, nil, utils.MakeErrorTrace(ctx, nil, "invalid type for indexing: %s", tp)
			}
			arrtp, ok = ptp.ElemType.(*types.ArrayType)
			if !ok {
				return nil, nil, utils.MakeErrorTrace(ctx, nil, "invalid type for indexing: %s", tp)
			}
		}
		return []value.Value{
			block.NewGetElementPtr(arrtp.ElemType, subexprs[0], idxs[0]),
		}, blocks, nil
	}
	switch s := ctx.GetChild(0).(type) {
	case parser.IPrimaryExprContext:
		return genCtx.GeneratePrimaryLValue(block, s)
	default:
		fmt.Println(ctx.GetText())
		return nil, nil, utils.MakeErrorTrace(ctx, nil, "this kind of lvalue not implemented")
	}
}

func (genCtx *GenContext) GeneratePrimaryLValue(block *ir.Block, ctx parser.IPrimaryExprContext) ([]value.Value, []*ir.Block, error) {
	if ctx.Operand() != nil {
		if ctx.Operand().OperandName() != nil {
			varName := ctx.Operand().OperandName().GetText()
			if varName == "_" {
				return []value.Value{nil}, nil, nil
			}
			if val, ok := genCtx.Vars.Lookup(varName); !ok {
				return nil, nil, utils.MakeErrorTrace(ctx, nil, "variable %s not defined in this scope", varName)
			} else {
				return []value.Value{val}, nil, nil
			}
		} else if ctx.Operand().L_PAREN() != nil {
			return genCtx.GenerateLValue(block, ctx.Operand().Expression())
		}
	} else if ctx.PrimaryExpr() != nil {
		if ctx.Arguments() != nil {
			// function call expected as rvalue - deference result pointer
			vals, blocks, err := genCtx.GeneratePrimaryExpr(block, ctx)
			if err != nil {
				return nil, nil, err
			} else if blocks != nil {
				block = blocks[len(blocks)-1]
			}
			if _, ok := vals[0].Type().(*types.PointerType); !ok {
				return nil, nil, utils.MakeErrorTrace(ctx, nil, "pointer type required for lvalue")
			}
			return []value.Value{vals[0], vals[0]}, blocks, nil
		} else if ctx.Index() != nil {
			// array indexing
			vals, blocks, err := genCtx.GeneratePrimaryLValue(block, ctx.PrimaryExpr())
			if err != nil {
				return nil, nil, utils.MakeErrorTrace(ctx, err, "faiiled to parse array indexing")
			} else if blocks != nil {
				block = blocks[len(blocks)-1]
			}
			idx, newBlocks, err := genCtx.GenerateExpr(block, ctx.Index().Expression())
			if err != nil {
				return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse array indexing")
			} else if newBlocks != nil {
				blocks = append(blocks, newBlocks...)
				block = blocks[len(blocks)-1]
			}
			tp := vals[0].Type()
			ptp, ok := tp.(*types.PointerType)
			if !ok {
				return nil, nil, utils.MakeErrorTrace(ctx, nil, "must be pointer type")
			}
			atp, ok := ptp.ElemType.(*types.ArrayType)
			if !ok {
				// try pointer-to-pointer-to-struct
				// to avoid syntax (*var).field
				ptptp, ok := ptp.ElemType.(*types.PointerType)
				if !ok {
					return nil, nil, utils.MakeErrorTrace(ctx, nil, "must be pointer type")
				}
				atp, ok = ptptp.ElemType.(*types.ArrayType)
				if !ok {
					return nil, nil, utils.MakeErrorTrace(ctx, nil, "must be pointer to array type")
				}
				ptp = ptptp
				vals[0] = block.NewLoad(ptptp, vals[0])
			}
			return []value.Value{
				// invalid source type when indexing array
				typesystem.NewTypedValue(
					block.NewGetElementPtr(ptp.ElemType, vals[0], constant.NewInt(types.I32, 0), idx[0]),
					types.NewPointer(atp.ElemType),
				),
			}, blocks, nil
		} else if ctx.DOT() != nil {
			// accessor to struct field
			vals, newBlocks, err := genCtx.GeneratePrimaryLValue(block, ctx.PrimaryExpr())
			if err != nil {
				return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse accessor")
			}
			tp := vals[0].Type()
			ptp, ok := tp.(*types.PointerType)
			if !ok {
				return nil, nil, utils.MakeErrorTrace(ctx, nil, "pointer type expected for lvalue")
			}
			stp, ok := ptp.ElemType.(*typesystem.StructInfo)
			if !ok {
				// try pointer-to-pointer-to-struct
				// to avoid syntax (*var).field
				ptptp, ok := ptp.ElemType.(*types.PointerType)
				if !ok {
					return nil, nil, utils.MakeErrorTrace(ctx, nil, "struct type expected for field accesor syntax")
				}
				stp, ok = ptptp.ElemType.(*typesystem.StructInfo)
				if !ok {
					return nil, nil, utils.MakeErrorTrace(ctx, nil, "struct type expected for field accesor syntax")
				}
				ptp = ptptp
				vals[0] = block.NewLoad(ptptp, vals[0])
			}
			fieldIdent := ctx.IDENTIFIER().GetText()
			offset, fieldType, err := stp.ComputeOffset(fieldIdent)
			if err != nil {
				return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to compute struct field offset")
			}
			elem, ok := ptp.ElemType.(*typesystem.StructInfo)
			if ok {
				tp = &elem.StructType
			} else {
				tp = ptp.ElemType
			}
			// generate GEP
			return []value.Value{
				typesystem.NewTypedValue(
					block.NewGetElementPtr(tp, vals[0], constant.NewInt(types.I32, 0), constant.NewInt(types.I32, int64(offset))),
					types.NewPointer(fieldType),
				),
			}, newBlocks, nil
		}
	}
	return nil, nil, utils.MakeErrorTrace(ctx, nil, "lvalue for primary expression not implemented")
}

func (genCtx *GenContext) GenerateIdentList(ctx parser.IIdentifierListContext) []string {
	var ids []string
	for i := range ctx.AllIDENTIFIER() {
		ids = append(ids, ctx.IDENTIFIER(i).GetText())
	}
	return ids
}

func (genCtx *GenContext) GenerateLValueList(block *ir.Block, ctx parser.IExpressionListContext) ([]value.Value, []*ir.Block, error) {
	var newBlocks []*ir.Block
	var lvals []value.Value
	for i := range ctx.AllExpression() {
		exprs, blocks, err := genCtx.GenerateLValue(block, ctx.Expression(i))
		if err != nil {
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse lvalue list")
		} else if blocks != nil {
			newBlocks = append(newBlocks, blocks...)
			block = newBlocks[len(newBlocks)-1]
		}
		lvals = append(lvals, exprs...)
	}
	return lvals, newBlocks, nil
}

func (genCtx *GenContext) GenerateExprList(block *ir.Block, ctx parser.IExpressionListContext) ([]value.Value, []*ir.Block, error) {
	var newBlocks []*ir.Block
	var vals []value.Value
	for _, c := range ctx.AllExpression() {
		exprs, blocks, err := genCtx.GenerateExpr(block, c)
		if err != nil {
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse expression list")
		} else if blocks != nil {
			newBlocks = append(newBlocks, blocks...)
			block = newBlocks[len(newBlocks)-1]
		}
		vals = append(vals, exprs...)
	}
	return vals, newBlocks, nil
}

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
		return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse expression")
	}
	right, rightBlocks, err := genCtx.GenerateExpr(block, ctx.Expression(1))
	_ = rightBlocks
	if err != nil {
		return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse expression")
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
			return nil, nil, utils.MakeErrorTrace(ctx, nil, "unimplemented instruction: %s", ctx.GetText())
		}
	} else if ctx.GetAdd_op() != nil {
		if ctx.PLUS() != nil {
			return genCtx.GenerateAddExpr(block, left[0], right[0])
		} else if ctx.MINUS() != nil {
			return genCtx.GenerateSubExpr(block, left[0], right[0])
		} else {
			return nil, nil, utils.MakeErrorTrace(ctx, nil, "unimplemented instruction: %s", ctx.GetText())
		}
	} else if ctx.GetRel_op() != nil {
		return genCtx.GenerateRelExpr(block, left[0], right[0], ctx)
	}

	return nil, nil, utils.MakeErrorTrace(ctx, nil, "other types of expression not implemented")
}

func (genCtx *GenContext) GeneratePrimaryExpr(block *ir.Block, ctx parser.IPrimaryExprContext) ([]value.Value, []*ir.Block, error) {
	if ctx.Operand() != nil {
		return genCtx.GenerateOperand(block, ctx.Operand())
	} else if ctx.Conversion() != nil {
		return nil, nil, utils.MakeErrorTrace(ctx, nil, "type conversions are not supported yet")
	} else if ctx.MethodExpr() != nil {
		return nil, nil, utils.MakeErrorTrace(ctx, nil, "method call expressions not supported yet")
	} else if ctx.PrimaryExpr() != nil {
		// function call
		if ctx.Arguments() != nil {
			exprs, blocks, err := genCtx.GeneratePrimaryExpr(block, ctx.PrimaryExpr())
			if err != nil {
				return nil, nil, err
			} else if blocks != nil {
				block = blocks[len(blocks)-1]
			}
			args, blocks, err := genCtx.GenerateArguments(block, ctx.Arguments())
			if err != nil {
				return nil, nil, err
			} else if blocks != nil {
				block = blocks[len(blocks)-1]
			}
			funRef, ok := exprs[0].(*ir.Func)
			if !ok {
				return nil, nil, utils.MakeErrorTrace(ctx, nil, "value %+v is not a func ref", exprs[0])
			}
			funDecl, err := genCtx.LookupFuncDeclByIR(funRef)
			if err != nil {
				return nil, nil, utils.MakeErrorTrace(ctx, nil, "function declaration for %s not found", funRef.String())
			}
			if funDecl.ReturnTypes == nil {
				block.NewCall(funRef, args...)
				return nil, blocks, nil
			} else if len(funDecl.ReturnTypes) == 1 {
				res := block.NewCall(funRef, args...)
				return []value.Value{typesystem.NewTypedValue(res, funRef.Sig.RetType)}, blocks, nil
			} else if len(funDecl.ReturnTypes) > 1 {
				// additional out parameters in front of explicit ones
				outParams := []value.Value{}
				for i := range funDecl.ReturnTypes {
					ref := block.NewAlloca(funRef.Params[i].Type().(*types.PointerType).ElemType)
					outParams = append(outParams, ref)
				}
				args = append(outParams, args...)
				block.NewCall(funRef, args...)
				resVals := []value.Value{}
				for _, ref := range outParams {
					resVals = append(resVals, block.NewLoad(ref.Type().(*types.PointerType).ElemType, ref))
				}
				return resVals, blocks, nil
			}
		} else if ctx.Index() != nil {
			// array indexing
			exprs, blocks, err := genCtx.GeneratePrimaryLValue(block, ctx.PrimaryExpr())
			if err != nil {
				return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse array indexing")
			} else if blocks != nil {
				block = blocks[len(blocks)-1]
			}
			idxs, newBlocks, err := genCtx.GenerateExpr(block, ctx.Index().Expression())
			if err != nil {
				return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse array index")
			} else if newBlocks != nil {
				blocks = append(blocks, newBlocks...)
				block = blocks[len(blocks)-1]
			}
			tp := exprs[0].Type()
			ptp, ok := tp.(*types.PointerType)
			if !ok {
				return nil, nil, utils.MakeErrorTrace(ctx, nil, "must be pointer type")
			}
			atp, ok := ptp.ElemType.(*types.ArrayType)
			if !ok {
				// try pointer-to-pointer-to-struct
				// to avoid syntax (*var).field
				ptptp, ok := ptp.ElemType.(*types.PointerType)
				if !ok {
					return nil, nil, utils.MakeErrorTrace(ctx, nil, "must be pointer type")
				}
				atp, ok = ptptp.ElemType.(*types.ArrayType)
				if !ok {
					return nil, nil, utils.MakeErrorTrace(ctx, nil, "must be pointer to array type")
				}
				ptp = ptptp
				exprs[0] = block.NewLoad(ptptp, exprs[0])
			}
			return []value.Value{
				typesystem.NewTypedValue(
					block.NewLoad(
						atp.ElemType,
						block.NewGetElementPtr(atp.ElemType, exprs[0], idxs[0]),
					),
					atp.ElemType,
				),
			}, blocks, nil
		} else if ctx.DOT() != nil {
			exprs, blocks, err := genCtx.GeneratePrimaryExpr(block, ctx.PrimaryExpr())
			if err != nil {
				return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse accessor syntax")
			} else if blocks != nil {
				block = blocks[len(blocks)-1]
			}
			// module name resolution
			if exprs[0].Type().Equal(typesystem.GoModuleType) {
				name := ctx.IDENTIFIER().GetText()
				val, err := genCtx.LookupNameInModule(exprs[0].(*typesystem.GoModule).Name, name)
				if err != nil {
					return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to resolve name %s in module %s", name, exprs[0].(*typesystem.GoModule).Name)
				}
				return []value.Value{val}, blocks, nil
			}
			// struct field accessor
			vals, newBlocks, err := genCtx.GeneratePrimaryLValue(block, ctx)
			if err != nil {
				return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse accessor")
			}
			// generate load
			return []value.Value{
				block.NewLoad(
					vals[0].Type().(*types.PointerType).ElemType,
					vals[0],
				),
			}, newBlocks, nil
		}
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
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse arguments")
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
		operandName := ctx.OperandName().IDENTIFIER().GetText()
		if val, ok := genCtx.Vars.Lookup(operandName); ok {
			elTp := val.Type().(*types.PointerType).ElemType
			return []value.Value{
				typesystem.NewTypedValue(
					block.NewLoad(elTp, val),
					elTp,
				),
			}, nil, nil
		}
		if module, ok := genCtx.PackageData.LookupModule(operandName); ok {
			return []value.Value{module}, nil, nil
		}
		if funRef, err := genCtx.LookupFunc(operandName); err == nil {
			return []value.Value{funRef}, nil, nil
		}
		return nil, nil, utils.MakeErrorTrace(ctx, nil, "name %s not defined in this scope", operandName)
	} else if ctx.Expression() != nil {
		return genCtx.GenerateExpr(block, ctx.Expression())
	}
	return nil, nil, utils.MakeErrorTrace(ctx, nil, "unmplemented operand")
}

func (genCtx *GenContext) GenerateLiteralExpr(block *ir.Block, ctx parser.ILiteralContext) ([]value.Value, []*ir.Block, error) {
	if ctx.BasicLit() != nil {
		return genCtx.GenerateBasicLiteralExpr(block, ctx.BasicLit())
	}
	return nil, nil, utils.MakeErrorTrace(ctx, nil, "unimplemented basic literal: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateBasicLiteralExpr(block *ir.Block, ctx parser.IBasicLitContext) ([]value.Value, []*ir.Block, error) {
	if ctx.NIL_LIT() != nil {
		return []value.Value{constant.NewNull(types.I32Ptr)}, nil, nil
	} else if ctx.Integer() != nil {
		val, err := constant.NewIntFromString(types.I32, ctx.Integer().GetText())
		if err != nil {
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse basic integer literal expression")
		}
		return []value.Value{val}, nil, nil
	} else if ctx.FLOAT_LIT() != nil {
		val, err := constant.NewFloatFromString(types.Double, ctx.FLOAT_LIT().GetText())
		if err != nil {
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse basic float literal expression")
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
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse basic string literal expression")
		}
		glob, ok := genCtx.Consts[strVal]
		if !ok {
			val := constant.NewCharArray(append([]byte(strVal), 0))
			glob = genCtx.module.NewGlobalDef(fmt.Sprintf("str.%d", len(genCtx.Consts)), val)
			genCtx.Consts[strVal] = glob
		}
		addr := constant.NewGetElementPtr(glob.ContentType, glob, constant.NewInt(types.I32, 0))
		return []value.Value{
			typesystem.NewTypedValue(addr, glob.Type()),
		}, nil, nil
	}
	return nil, nil, utils.MakeErrorTrace(ctx, nil, "not implemented basic literal: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateUnaryExpr(block *ir.Block, ctx parser.IExpressionContext) ([]value.Value, []*ir.Block, error) {
	if ctx.PLUS() != nil {
		return genCtx.GenerateExpr(block, ctx.Expression(0))
	} else if ctx.MINUS() != nil {
		vals, blocks, err := genCtx.GenerateExpr(block, ctx.Expression(0))
		if err != nil {
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to generate unary expression")
		} else if blocks != nil {
			block = blocks[len(blocks)-1]
		}
		tp := vals[0].Type()
		if typesystem.IsIntType(tp) {
			return []value.Value{
				block.NewSub(constant.NewInt(tp.(*types.IntType), 0), vals[0]),
			}, blocks, nil
		} else if typesystem.IsFloatType(tp) {
			return []value.Value{
				block.NewFSub(constant.NewFloat(tp.(*types.FloatType), 0), vals[0]),
			}, blocks, nil
		} else {
			return nil, nil, utils.MakeErrorTrace(ctx, nil, "unsupported type for unary minus: %s", tp.String())
		}
	} else if ctx.EXCLAMATION() != nil {
		vals, blocks, err := genCtx.GenerateExpr(block, ctx.Expression(0))
		if err != nil {
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse unary expression")
		} else if blocks != nil {
			block = blocks[len(blocks)-1]
		}
		return []value.Value{
			typesystem.NewTypedValue(
				block.NewXor(vals[0], constant.True),
				typesystem.Bool,
			),
		}, blocks, nil
	} else if ctx.AMPERSAND() != nil {
		// only taking address from variable
		exprs, blocks, err := genCtx.GenerateLValue(block, ctx.Expression(0))
		if err != nil {
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse unary expression")
		}
		return []value.Value{exprs[0]}, blocks, nil
	} else if ctx.STAR() != nil {
		lvals, blocks, err := genCtx.GenerateLValue(block, ctx.Expression(0))
		if err != nil {
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse unary expression")
		}
		varRef := lvals[0]
		ptrtp, ok := varRef.Type().(*types.PointerType)
		if !ok {
			return nil, nil, utils.MakeErrorTrace(ctx, nil, "bad lvalue. Expected pointer type")
		}
		ptrtp2, ok := ptrtp.ElemType.(*types.PointerType)
		if !ok {
			return nil, nil, utils.MakeErrorTrace(ctx, nil, "not pointer type dereference")
		}
		return []value.Value{
			typesystem.NewTypedValue(
				block.NewLoad(ptrtp2.ElemType, block.NewLoad(ptrtp2, varRef)),
				ptrtp2.ElemType,
			),
		}, blocks, nil
	}
	return nil, nil, utils.MakeErrorTrace(ctx, nil, "unimplemented unary expression: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateMulExpr(block *ir.Block, left, right value.Value) ([]value.Value, []*ir.Block, error) {
	if resType, ok := typesystem.CommonSupertype(left, right); !ok {
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
	if resType, ok := typesystem.CommonSupertype(left, right); !ok {
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
	if resType, ok := typesystem.CommonSupertype(left, right); !ok {
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
	if resType, ok := typesystem.CommonSupertype(left, right); !ok {
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
	if resType, ok := typesystem.CommonSupertype(left, right); !ok {
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
	resType, ok := typesystem.CommonSupertype(left, right)
	if !ok {
		return nil, nil, utils.MakeErrorTrace(ctx, nil, "failed to deduce common type for %v and %v", left.Type(), right.Type())
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
				cmpPred = enum.IPredSLT
			} else {
				cmpPred = enum.IPredULT
			}
		} else if ctx.LESS_OR_EQUALS() != nil {
			if _, ok := resType.(*types.IntType); ok {
				cmpPred = enum.IPredSLE
			} else {
				// TODO: fix unsigned int handling
				cmpPred = enum.IPredULE
			}
		} else if ctx.GREATER() != nil {
			if _, ok := resType.(*types.IntType); ok {
				cmpPred = enum.IPredSGT
			} else {
				cmpPred = enum.IPredUGT
			}
		} else if ctx.GREATER_OR_EQUALS() != nil {
			if _, ok := resType.(*types.IntType); ok {
				cmpPred = enum.IPredSGE
			} else {
				cmpPred = enum.IPredUGE
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
		return nil, nil, utils.MakeError("left value not of type bool: (got %v)", left.Type())
	}
	if !right.Type().Equal(typesystem.Bool) {
		return nil, nil, utils.MakeError("right value not of type bool: (got %v)", left.Type())
	}
	return []value.Value{
		typesystem.NewTypedValue(block.NewAnd(left, right), typesystem.Bool),
	}, nil, nil
}

func (genCtx *GenContext) GenerateOrExpr(block *ir.Block, left, right value.Value) ([]value.Value, []*ir.Block, error) {
	if !left.Type().Equal(typesystem.Bool) {
		return nil, nil, utils.MakeError("left value not of type bool: (got %v)", left.Type())
	}
	if !right.Type().Equal(typesystem.Bool) {
		return nil, nil, utils.MakeError("right value not of type bool: (got %v)", left.Type())
	}
	return []value.Value{
		typesystem.NewTypedValue(block.NewOr(left, right), typesystem.Bool),
	}, nil, nil
}

// helper function for debugging to print out current context state (position)
func PrintCurrentState(ctx *antlr.BaseParserRuleContext) {
	fmt.Println(ctx.GetText())
}
