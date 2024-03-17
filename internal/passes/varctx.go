package passes

import (
	"gocomp/internal/utils"

	"github.com/llir/llvm/ir/value"
)

type VariableContext struct {
	Parent *VariableContext
	vars   map[string]value.Value
}

func NewVarContext(parent *VariableContext) *VariableContext {
	return &VariableContext{
		Parent: parent,
		vars:   make(map[string]value.Value),
	}
}

func (ctx *VariableContext) Lookup(name string) (value.Value, bool) {
	v, ok := ctx.vars[name]
	if ok || ctx.Parent == nil {
		return v, ok
	} else {
		return ctx.Parent.Lookup(name)
	}
}

func (ctx *VariableContext) Add(name string, val value.Value) error {
	if _, ok := ctx.vars[name]; ok {
		return utils.MakeError("variable %s already defined in current scope", name)
	}
	ctx.vars[name] = val
	return nil
}
