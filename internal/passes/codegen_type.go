package passes

import (
	"gocomp/internal/parser"
	"gocomp/internal/typesystem"
	"gocomp/internal/utils"
	"strconv"

	"github.com/llir/llvm/ir/types"
)

func ParseType(ctx parser.IType_Context) (types.Type, error) {
	if ctx.L_PAREN() != nil {
		return ParseType(ctx.Type_())
	} else if ctx.TypeName() != nil {
		return typesystem.GoTypeToIR(ctx.TypeName().GetText())
	} else {
		switch tp := ctx.TypeLit().GetChild(0).(type) {
		case parser.IArrayTypeContext:
			return ParseArrayType(tp)
		case parser.IPointerTypeContext:
			return ParsePointerType(tp)
		}
	}
	return nil, utils.MakeError("failed to parse type: %s", ctx.GetText())
}

func ParsePointerType(ctx parser.IPointerTypeContext) (types.Type, error) {
	if underlying, err := ParseType(ctx.Type_()); err != nil {
		return nil, err
	} else {
		return types.NewPointer(underlying), nil
	}
}

func ParseArrayType(ctx parser.IArrayTypeContext) (types.Type, error) {
	len, err := strconv.Atoi(ctx.ArrayLength().GetText())
	if err != nil {
		return nil, err
	} else if len < 0 {
		return nil, utils.MakeError("negative array length not allowed")
	} else if underlying, err := ParseType(ctx.ElementType().Type_()); err != nil {
		return nil, err
	} else {
		return types.NewArray(uint64(len), underlying), nil
	}
}
