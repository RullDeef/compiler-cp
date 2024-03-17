package typesystem

import (
	"gocomp/internal/utils"

	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type UintType struct {
	types.IntType
}

var (
	Bool    = types.I1
	Int8    = types.I8
	Int16   = types.I16
	Int32   = types.I32
	Int64   = types.I64
	Uint8   = &UintType{IntType: *types.I8}
	Uint16  = &UintType{IntType: *types.I16}
	Uint32  = &UintType{IntType: *types.I32}
	Uint64  = &UintType{IntType: *types.I64}
	Float32 = types.Float
	Float64 = types.Double
	// Complex64
	// Complex128
	Uintptr = types.I8Ptr
	Byte    = Uint8
	Rune    = Int32
	Int     = Int32
	Uint    = Uint32
)

type TypedValue struct {
	value.Value
	type_ types.Type
}

func NewTypedValue(value value.Value, tp types.Type) *TypedValue {
	return &TypedValue{
		Value: value,
		type_: tp,
	}
}

func (nv TypedValue) Type() types.Type {
	return nv.type_
}

func IsBoolType(t types.Type) bool {
	return t == Bool
}

func IsIntType(t types.Type) bool {
	_, okUint := t.(*UintType)
	_, okInt := t.(*types.IntType)
	return !okUint && okInt
}

func IsUintType(t types.Type) bool {
	_, ok := t.(*UintType)
	return ok
}

func IsFloatType(t types.Type) bool {
	_, ok := t.(*types.FloatType)
	return ok
}

func CommonSupertype(t1, t2 types.Type) (types.Type, bool) {
	if t1 == t2 {
		return t1, true
	}

	return nil, false
}

func GoTypeToIR(goType string) (types.Type, error) {
	if goType == "" {
		return nil, utils.MakeError("emtpy go type")
	}
	if goType[:1] == "*" {
		underlying, err := GoTypeToIR(goType[1:])
		if err != nil {
			return nil, err
		}
		return types.NewPointer(underlying), nil
	}
	t, ok := map[string]types.Type{
		"":        types.Void,
		"bool":    types.I1,
		"int8":    types.I8,
		"int16":   types.I16,
		"int32":   types.I32,
		"int64":   types.I64,
		"uint8":   types.I8,
		"uint16":  types.I16,
		"uint32":  types.I32,
		"uint64":  types.I64,
		"int":     types.I32,
		"uint":    types.I32,
		"float32": types.Float,
		"float64": types.Double,
	}[goType]
	if !ok {
		return nil, utils.MakeError("invalid primitive type: %s", goType)
	}
	return t, nil
}
