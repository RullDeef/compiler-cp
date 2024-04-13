package passes

import (
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/typesystem"
	"gocomp/internal/utils"
	"strconv"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
)

type typeManager struct {
	// aliases to base types, created with 'type' keyword
	userAliases map[string]types.Type
	// struct types created with 'type' keyword
	userStructs map[string]*typesystem.StructInfo
}

func newTypeManager() *typeManager {
	return &typeManager{
		userAliases: make(map[string]types.Type),
		userStructs: make(map[string]*typesystem.StructInfo),
	}
}

func (m *typeManager) UpdateModule(module *ir.Module) {
	for _, tp := range m.userAliases {
		module.TypeDefs = append(module.TypeDefs, tp)
	}
	for name, tp := range m.userStructs {
		lltp := tp
		lltp.TypeName = name
		module.TypeDefs = append(module.TypeDefs, lltp)
	}
}

func (m *typeManager) ParseTypeDecl(ctx parser.ITypeDeclContext) error {
	for _, spec := range ctx.AllTypeSpec() {
		if err := m.ParseTypeSpec(spec); err != nil {
			return err
		}
	}
	return nil
}

func (m *typeManager) ParseTypeSpec(ctx parser.ITypeSpecContext) error {
	if ctx.AliasDecl() != nil {
		return m.ParseAliasDecl(ctx.AliasDecl())
	} else if ctx.TypeDef() != nil {
		return m.ParseTypeDef(ctx.TypeDef())
	}
	return utils.MakeError("must never happen")
}

func (m *typeManager) ParseAliasDecl(ctx parser.IAliasDeclContext) error {
	name := ctx.IDENTIFIER().GetText()
	tp, err := m.ParseType(ctx.Type_())
	if err != nil {
		return err
	}
	if _, ok := m.userAliases[name]; ok {
		return utils.MakeError(fmt.Sprintf("type %s already defined as alias to primary type", name))
	}
	if _, ok := m.userStructs[name]; ok {
		return utils.MakeError(fmt.Sprintf("type %s already defined as alias to struct type", name))
	}
	if stp, ok := tp.(*typesystem.StructInfo); ok {
		m.userStructs[name] = stp
	} else {
		m.userAliases[name] = tp
	}
	return nil
}

func (m *typeManager) ParseTypeDef(ctx parser.ITypeDefContext) error {
	name := ctx.IDENTIFIER().GetText()
	if _, ok := m.userAliases[name]; ok {
		return utils.MakeError(fmt.Sprintf("type %s already defined as alias to primary type", name))
	}
	if _, ok := m.userStructs[name]; ok {
		return utils.MakeError(fmt.Sprintf("type %s already defined as alias to struct type", name))
	}
	// look ahead for recursive struct parsing
	tmpInfo := &typesystem.StructInfo{}
	tmpInfo.TypeName = name
	m.userStructs[name] = tmpInfo

	tp, err := m.ParseType(ctx.Type_())
	if err != nil {
		return err
	}
	if stp, ok := tp.(*typesystem.StructInfo); ok {
		stp.SetName(name)
		m.userStructs[name] = stp
		stp.UpdateRecursiveRef(tmpInfo)
	} else {
		m.userAliases[name] = tp
	}
	return nil
}

func (m *typeManager) ParseType(ctx parser.IType_Context) (types.Type, error) {
	if ctx.L_PAREN() != nil {
		return m.ParseType(ctx.Type_())
	} else if ctx.TypeName() != nil {
		typename := ctx.TypeName().GetText()
		tp, ok := m.userAliases[typename]
		if ok {
			return tp, nil
		}
		tp, ok = m.userStructs[typename]
		if ok {
			return tp, nil
		}
		tp, err := typesystem.GoTypeToIR(typename)
		if err == nil {
			return tp, nil
		}
	} else {
		switch tp := ctx.TypeLit().GetChild(0).(type) {
		case parser.IArrayTypeContext:
			return m.ParseArrayType(tp)
		case parser.IPointerTypeContext:
			return m.ParsePointerType(tp)
		case parser.IStructTypeContext:
			return m.ParseStructType(tp)
		}
	}
	return nil, utils.MakeError("failed to parse type: %s", ctx.GetText())
}

func (m *typeManager) ParsePointerType(ctx parser.IPointerTypeContext) (types.Type, error) {
	if underlying, err := m.ParseType(ctx.Type_()); err != nil {
		return nil, err
	} else {
		return types.NewPointer(underlying), nil
	}
}

func (m *typeManager) ParseArrayType(ctx parser.IArrayTypeContext) (types.Type, error) {
	len, err := strconv.Atoi(ctx.ArrayLength().GetText())
	if err != nil {
		return nil, err
	} else if len < 0 {
		return nil, utils.MakeError("negative array length not allowed")
	} else if underlying, err := m.ParseType(ctx.ElementType().Type_()); err != nil {
		return nil, err
	} else {
		return types.NewArray(uint64(len), underlying), nil
	}
}

func (m *typeManager) ParseStructType(ctx parser.IStructTypeContext) (types.Type, error) {
	fields := []typesystem.StructFieldInfo{}
	offset := 0
	for _, field := range ctx.AllFieldDecl() {
		if field.EmbeddedField() != nil {
			return nil, utils.MakeError("embedded fields not supported yet")
		}
		for _, ident := range field.IdentifierList().AllIDENTIFIER() {
			fieldType, err := m.ParseType(field.Type_())
			if err != nil {
				return nil, err
			}
			if stp, ok := fieldType.(*typesystem.StructInfo); ok {
				fields = append(fields, typesystem.StructFieldInfo{
					Name:     ident.GetText(),
					Offset:   offset,
					IsStruct: true,
					Struct:   stp,
				})
			} else {
				fields = append(fields, typesystem.StructFieldInfo{
					Name:      ident.GetText(),
					Offset:    offset,
					Primitive: fieldType,
				})
			}
			offset += 1
		}
	}
	return typesystem.NewStructInfo("", fields), nil
}
