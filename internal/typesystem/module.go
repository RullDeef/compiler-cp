package typesystem

import (
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type goModuleType struct {
}

var _ types.Type = new(goModuleType)
var GoModuleType = &goModuleType{}

func (*goModuleType) String() string {
	return ""
}

func (*goModuleType) LLString() string {
	return ""
}

func (gmt *goModuleType) Equal(u types.Type) bool {
	return gmt == GoModuleType
}

func (gmt *goModuleType) Name() string {
	return ""
}

func (gmt *goModuleType) SetName(name string) {
}

type GoModule struct {
	Name string
}

var _ value.Value = new(GoModule)

func (gm *GoModule) String() string {
	return gm.Name
}

func (gm *GoModule) Type() types.Type {
	return GoModuleType
}

func (gm *GoModule) Ident() string {
	return gm.Name
}
