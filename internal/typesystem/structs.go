package typesystem

import (
	"fmt"
	"gocomp/internal/utils"

	"github.com/llir/llvm/ir/types"
)

type StructInfo struct {
	types.StructType

	TypeName string

	Fields []StructFieldInfo
}

type StructFieldInfo struct {
	Name   string
	Offset int
	/* IsEmbedded bool */
	IsStruct  bool
	Struct    *StructInfo
	Primitive types.Type
}

func NewStructInfo(name string, fields []StructFieldInfo) *StructInfo {
	si := StructInfo{
		TypeName: name,
		Fields:   fields,
	}
	llvmFields := make([]types.Type, 0)
	for _, field := range fields {
		if field.IsStruct {
			tp := field.Struct
			llvmFields = append(llvmFields, tp)
		} else {
			llvmFields = append(llvmFields, field.Primitive)
		}
	}
	si.StructType = *types.NewStruct(llvmFields...)
	si.StructType.TypeName = si.TypeName
	return &si
}

func (si *StructInfo) String() string {
	return fmt.Sprintf("%%%s", si.TypeName)
}

func (si *StructInfo) SetName(name string) {
	si.StructType.SetName(name)
	si.TypeName = name
}

// Equal reports whether t and u are of equal type.
func (si *StructInfo) Equal(u types.Type) bool {
	return si == u
}

func (si *StructInfo) UpdateRecursiveRef(ref *StructInfo) {
	for i, field := range si.Fields {
		if field.IsStruct && field.Struct == ref {
			si.Fields[i].Struct = si
		} else if ptp, ok := field.Primitive.(*types.PointerType); ok {
			if ptp.ElemType == ref {
				si.Fields[i].Primitive = types.NewPointer(si)
			}
		}
	}
}

func (si *StructInfo) Size() (int64, error) {
	size := int64(0)
	for _, field := range si.Fields {
		s, err := field.Size()
		if err != nil {
			return 0, err
		}
		size += s
	}
	return size, nil
}

func (si *StructInfo) ComputeOffset(fieldName string) (int, types.Type, error) {
	for _, field := range si.Fields {
		if field.Name == fieldName {
			if field.IsStruct {
				return field.Offset, field.Struct, nil
			} else {
				return field.Offset, field.Primitive, nil
			}
		}
	}
	return 0, nil, utils.MakeError(fmt.Sprintf("field %s not found in type %s", fieldName, si.TypeName))
}

func (sf *StructFieldInfo) Size() (int64, error) {
	if sf.IsStruct {
		return sf.Struct.Size()
	} else {
		return primitiveSize(sf.Primitive)
	}
}

func primitiveSize(tp types.Type) (int64, error) {
	if intg, ok := tp.(*types.IntType); ok {
		return int64(intg.BitSize) / 8, nil
	} else if flt, ok := tp.(*types.FloatType); ok {
		switch flt.Kind {
		// 32-bit floating-point type (IEEE 754 single precision).
		case types.FloatKindFloat:
			return 4, nil
		// 64-bit floating-point type (IEEE 754 double precision).
		case types.FloatKindDouble:
			return 8, nil
		}
	} else if arr, ok := tp.(*types.ArrayType); ok {
		elSize, err := primitiveSize(arr.ElemType)
		if err != nil {
			return 0, err
		}
		return int64(arr.Len) * elSize, nil
	} else if _, ok := tp.(*types.PointerType); ok {
		return 8, nil
	}
	return 0, utils.MakeError("cannot compute size of type %v", tp)
}
