package typesystem

import (
	"fmt"

	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type BasicType int

const (
	Nil = BasicType(iota)
	Bool
	Int8
	Int16
	Int32
	Int64
	Uint8
	Uint16
	Uint32
	Uint64
	Float32
	Float64
	// Complex64
	// Complex128
	Uintptr
	Byte = Uint8
	Rune = Int32
	Int  = Int32
	Uint = Uint32
)

// todo: replace with value.Named
type TypedValue struct {
	Value value.Value
	Type  any
}

func NewTypedValueFromIR(irt types.Type, val value.Named) *TypedValue {
	var basicType BasicType
	if irt == types.I1 {
		basicType = Bool
	} else if irt == types.I8 {
		basicType = Int8
	} else if irt == types.I16 {
		basicType = Int16
	} else if irt == types.I32 {
		basicType = Int32
	} else if irt == types.I64 {
		basicType = Int64
	} else if irt == types.Float {
		basicType = Float32
	} else if irt == types.Double {
		basicType = Float64
	}
	return &TypedValue{
		Value: val,
		Type:  basicType,
	}
}

type ArrayType struct {
	Length         uint64
	UnderlyingType any
}

type SliceType struct {
	UnderlyingType any
}

type StructType struct {
	TypeName string
	Fields   []StructFieldType
}

type StructFieldType struct {
	Name           string
	UnderlyingType any
}

func IsIntType(t BasicType) bool {
	return t >= Int8 && t <= Int64
}

func IsUintType(t BasicType) bool {
	return t >= Uint8 && t <= Uint64
}

func IsFloatType(t BasicType) bool {
	return t == Float32 || t == Float64
}

func CommonSupertype(t1, t2 any) (BasicType, bool) {
	bt1, ok := t1.(BasicType)
	if !ok {
		return 0, false
	}
	bt2, ok := t2.(BasicType)
	if !ok {
		return 0, false
	}
	if bt1 == bt2 {
		return bt1, true
	} else if IsIntType(bt1) && IsIntType(bt2) {
		return max(bt1, bt2), true
	} else if IsFloatType(bt1) && IsFloatType(bt2) {
		return max(bt1, bt2), true
	}
	return 0, false
}

func (tp *TypedValue) LLVMType() (types.Type, error) {
	btp, ok := tp.Type.(BasicType)
	if !ok {
		return nil, fmt.Errorf("not basic type")
	}
	switch btp {
	case Bool:
		return types.I1, nil
	case Int8:
		return types.I8, nil
	case Int16:
		return types.I16, nil
	case Int32:
		return types.I32, nil
	case Int64:
		return types.I64, nil
	case Uint8:
		return types.I8, nil
	case Uint16:
		return types.I16, nil
	case Uint32:
		return types.I32, nil
	case Uint64:
		return types.I64, nil
	case Float32:
		return types.Float, nil
	case Float64:
		return types.Double, nil
	}
	return nil, fmt.Errorf("invalid type")
}
