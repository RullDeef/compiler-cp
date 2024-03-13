package typesystem

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
