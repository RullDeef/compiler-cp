package passes

import (
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/typesystem"
	"gocomp/internal/utils"
	"strconv"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type keyedElement struct {
	key     string
	element value.Value
}

func (genCtx *GenContext) GenerateLiteralExpr(block *ir.Block, ctx parser.ILiteralContext) ([]value.Value, []*ir.Block, error) {
	if ctx.BasicLit() != nil {
		return genCtx.GenerateBasicLiteralExpr(block, ctx.BasicLit())
	} else if ctx.CompositeLit() != nil {
		return genCtx.GenerateCompositeLiteralExpr(block, ctx.CompositeLit())
	}
	return nil, nil, utils.MakeErrorTrace(ctx, nil, "unimplemented basic literal: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateBasicLiteralExpr(block *ir.Block, ctx parser.IBasicLitContext) ([]value.Value, []*ir.Block, error) {
	if ctx.NIL_LIT() != nil {
		return []value.Value{constant.NewNull(types.I32Ptr)}, nil, nil
	} else if ctx.Integer() != nil {
		istr := ctx.Integer().GetText()
		// adjust for hexadecimals
		if len(istr) >= 2 && (istr[:2] == "0x" || istr[:2] == "0X") {
			istr = "s" + istr
		}
		val, err := constant.NewIntFromString(types.I32, istr)
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
			typesystem.NewTypedValue(addr, types.I8Ptr),
		}, nil, nil
	}
	return nil, nil, utils.MakeErrorTrace(ctx, nil, "not implemented basic literal: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateCompositeLiteralExpr(block *ir.Block, ctx parser.ICompositeLitContext) ([]value.Value, []*ir.Block, error) {
	// parse literal type
	ltp, err := genCtx.PackageData.typeManager.ParseLiteralType(ctx.LiteralType())
	if err != nil {
		return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to generate composite literal expression")
	}
	// parse value
	val, blocks, err := genCtx.GenerateCompositeLiteralValue(block, ltp, ctx.LiteralValue())
	if err != nil {
		return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to generate composite literal expression")
	}
	return []value.Value{val}, blocks, nil
}

func (genCtx *GenContext) GenerateCompositeLiteralValue(block *ir.Block, tp types.Type, ctx parser.ILiteralValueContext) (value.Value, []*ir.Block, error) {
	if stp, ok := tp.(*typesystem.StructInfo); ok {
		return genCtx.GenerateStructLiteralValue(block, stp, ctx)
	}
	if atp, ok := tp.(*types.ArrayType); ok {
		return genCtx.GenerateArrayLiteralValue(block, atp, ctx)
	}
	return nil, nil, utils.MakeErrorTrace(ctx, nil, "unimplemented composite literal value: %s", ctx.GetText())
}

func (genCtx *GenContext) GenerateStructLiteralValue(block *ir.Block, stp *typesystem.StructInfo, ctx parser.ILiteralValueContext) (value.Value, []*ir.Block, error) {
	slitAddr := block.NewAlloca(stp)
	block.NewStore(constant.NewZeroInitializer(stp), slitAddr)
	if ctx.ElementList() == nil {
		return block.NewLoad(stp, slitAddr), nil, nil
	}
	keyedElems := []keyedElement{}
	var blocks []*ir.Block
	for _, kElemCtx := range ctx.ElementList().AllKeyedElement() {
		kelem, newBlocks, err := genCtx.ParseKeyedElement(block, stp, kElemCtx)
		if err != nil {
			return nil, nil, err
		} else if newBlocks != nil {
			blocks = append(blocks, newBlocks...)
			block = blocks[len(blocks)-1]
		}
		// check for duplicate key names
		for _, k := range keyedElems {
			if k.key == kelem.key {
				return nil, nil, utils.MakeErrorTrace(ctx, nil, "duplicate field name in struct literal")
			}
		}
		// build up struct value
		var off int
		if kelem.key != "" {
			var tp types.Type
			var err error
			off, tp, err = stp.ComputeOffset(kelem.key)
			if err != nil {
				return nil, nil, err
			}
			// TODO: check types for tp and keyed element
			_ = tp
		} else {
			// offset by field position in literal
			off = len(keyedElems)
			kelem.key = stp.Fields[off].Name
		}
		// TODO: fix this hack for nil values
		if nilElem, ok := kelem.element.(*constant.Null); ok {
			nilElem.Typ = stp.Fields[off].Primitive.(*types.PointerType)
		}
		block.NewStore(
			kelem.element,
			block.NewGetElementPtr(&stp.StructType, slitAddr, constant.NewInt(types.I32, 0), constant.NewInt(types.I32, int64(off))),
		)
		keyedElems = append(keyedElems, *kelem)
	}
	slitVal := block.NewLoad(stp, slitAddr)
	return slitVal, blocks, nil
}

func (genCtx *GenContext) GenerateArrayLiteralValue(block *ir.Block, atp *types.ArrayType, ctx parser.ILiteralValueContext) (value.Value, []*ir.Block, error) {
	alitAddr := block.NewAlloca(atp)
	block.NewStore(constant.NewZeroInitializer(atp), alitAddr)
	if ctx.ElementList() == nil {
		return block.NewLoad(atp, alitAddr), nil, nil
	}
	initedIndices := []int{}
	var blocks []*ir.Block
	i := 0
	for _, kElemCtx := range ctx.ElementList().AllKeyedElement() {
		kelem, newBlocks, err := genCtx.ParseKeyedElement(block, atp, kElemCtx)
		if err != nil {
			return nil, nil, err
		} else if newBlocks != nil {
			blocks = append(blocks, newBlocks...)
			block = blocks[len(blocks)-1]
		}
		if kelem.key != "" {
			ki, err := strconv.ParseInt(kelem.key, 10, 64)
			if err != nil {
				return nil, nil, utils.MakeErrorTrace(ctx, nil, "invalid string for array index: %s", kelem.key)
			}
			i = int(ki)
		}
		if i < 0 || i >= int(atp.Len) {
			return nil, nil, utils.MakeErrorTrace(ctx, nil, "literal array index out of bound")
		}
		// check for duplicate array indices
		for _, ind := range initedIndices {
			if ind == i {
				return nil, nil, utils.MakeErrorTrace(ctx, nil, "duplicate index in array literal")
			}
		}
		// build up array value
		// TODO: check types for array element and keyed element
		block.NewStore(
			kelem.element,
			block.NewGetElementPtr(atp, alitAddr, constant.NewInt(types.I32, 0), constant.NewInt(types.I32, int64(i))),
		)
		initedIndices = append(initedIndices, i)
		i++
	}
	alitVal := block.NewLoad(atp, alitAddr)
	return alitVal, blocks, nil
}

func (genCtx *GenContext) ParseKeyedElement(block *ir.Block, parentType types.Type, ctx parser.IKeyedElementContext) (*keyedElement, []*ir.Block, error) {
	key := ""
	if ctx.COLON() != nil {
		key = ctx.Key().GetText()
	}
	if ctx.Element().LiteralValue() != nil {
		// if parent type is array or struct - handle element types accordingly
		if atp, ok := parentType.(*types.ArrayType); ok {
			etp := atp.ElemType
			element, blocks, err := genCtx.GenerateCompositeLiteralValue(block, etp, ctx.Element().LiteralValue())
			if err != nil {
				return nil, nil, err
			}
			return &keyedElement{key: key, element: element}, blocks, nil
		} else if stp, ok := parentType.(*typesystem.StructInfo); ok {
			// check if key exists and references existing struct field
			if key == "" {
				return nil, nil, utils.MakeErrorTrace(ctx, nil, "implicit keys not supported in struct literals")
			}
			off, tp, err := stp.ComputeOffset(key)
			if err != nil {
				return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to compute struct field offset")
			}
			_, _ = off, tp
			return nil, nil, utils.MakeErrorTrace(ctx, nil, "nested struct literal values not supported yet")
		} else {
			return nil, nil, utils.MakeErrorTrace(ctx, nil, "invalid parent type")
		}
	} else {
		exprs, blocks, err := genCtx.GenerateExpr(block, ctx.Element().Expression())
		if err != nil {
			return nil, nil, err
		}
		return &keyedElement{
			key:     key,
			element: exprs[0],
		}, blocks, nil
	}
}
