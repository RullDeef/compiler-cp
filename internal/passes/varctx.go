package passes

import (
	"fmt"
	"gocomp/internal/typesystem"
)

type VariableContext struct {
	Parent *VariableContext
	vars   map[string]*typesystem.TypedValue
}

func NewVarContext(parent *VariableContext) *VariableContext {
	return &VariableContext{
		Parent: parent,
		vars:   make(map[string]*typesystem.TypedValue),
	}
}

func (ctx *VariableContext) Lookup(name string) (*typesystem.TypedValue, bool) {
	v, ok := ctx.vars[name]
	if ok || ctx.Parent == nil {
		return v, ok
	} else {
		return ctx.Parent.Lookup(name)
	}
}

func (ctx *VariableContext) Add(name string, val *typesystem.TypedValue) error {
	if _, ok := ctx.vars[name]; ok {
		return fmt.Errorf("variable %s already defined in current scope", name)
	}
	ctx.vars[name] = val
	return nil
}
