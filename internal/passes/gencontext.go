package passes

import (
	"fmt"
	"gocomp/internal/utils"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type GenContext struct {
	PackageData *PackageData

	module           *ir.Module
	Funcs            map[string]*ir.Func
	SpecialFuncs     map[string]*ir.Func
	SpecialFuncDecls map[string]*FunctionDecl
	Consts           map[string]*ir.Global

	// global variable context
	Vars *VariableContext
}

func NewGenContext(pdata *PackageData) (*GenContext, error) {
	ctx := GenContext{
		PackageData:      pdata,
		module:           ir.NewModule(),
		Funcs:            make(map[string]*ir.Func),
		SpecialFuncs:     make(map[string]*ir.Func),
		SpecialFuncDecls: make(map[string]*FunctionDecl),
		Consts:           make(map[string]*ir.Global),
		Vars:             NewVarContext(nil),
	}

	// populate global functions (like printf)
	fun := ir.NewFunc("printf", types.I32, ir.NewParam("format", types.I8Ptr))
	fun.Sig.Variadic = true
	ctx.SpecialFuncs["fmt__Printf"] = fun
	ctx.SpecialFuncDecls["fmt__Printf"] = &FunctionDecl{
		Name:        "fmt__Printf",
		ArgNames:    []string{"format"},
		ArgTypes:    []types.Type{types.I8Ptr},
		ReturnTypes: []types.Type{types.I32},
	}

	fun = ir.NewFunc("scanf", types.I32, ir.NewParam("format", types.I8Ptr))
	fun.Sig.Variadic = true
	ctx.SpecialFuncs["fmt__Scanf"] = fun
	ctx.SpecialFuncDecls["fmt__Scanf"] = &FunctionDecl{
		Name:        "fmt__Scanf",
		ArgNames:    []string{"format"},
		ArgTypes:    []types.Type{types.I8Ptr},
		ReturnTypes: []types.Type{types.I32},
	}

	// generate references to functions first
	for _, fn := range pdata.Functions {
		irFun, err := genFunDef(fn)
		if err != nil {
			return nil, err
		}
		irFun.Parent = ctx.module
		ctx.Funcs[fn.Name] = irFun
	}

	return &ctx, nil
}

func (ctx *GenContext) Module() *ir.Module {
	// link all function defs
	if len(ctx.module.Funcs) == 0 {
		for _, fun := range ctx.SpecialFuncs {
			fun.Parent = ctx.module
			ctx.module.Funcs = append(ctx.module.Funcs, fun)
		}
		for _, fun := range ctx.Funcs {
			fun.Parent = ctx.module
			ctx.module.Funcs = append(ctx.module.Funcs, fun)
		}
	}
	return ctx.module
}

func (ctx *GenContext) PushLexicalScope() {
	ctx.Vars = NewVarContext(ctx.Vars)
}

func (ctx *GenContext) PopLexicalScope() {
	ctx.Vars = ctx.Vars.Parent
}

func (ctx *GenContext) LookupNameInModule(moduleName, name string) (value.Value, error) {
	// checks only functions for now
	if fun, err := ctx.LookupFunc(fmt.Sprintf("%s__%s", moduleName, name)); err != nil {
		return nil, err
	} else {
		return fun, nil
	}
}

func (ctx *GenContext) LookupFuncDecl(funName string) (*FunctionDecl, error) {
	if f, ok := ctx.SpecialFuncDecls[funName]; ok {
		return f, nil
	}
	packageFunName := ctx.PackageData.PackageName + "__" + funName
	if f, ok := ctx.PackageData.Functions[packageFunName]; ok {
		return f, nil
	}
	return nil, utils.MakeError("function %s not defined", funName)
}

func (ctx *GenContext) LookupFunc(funName string) (*ir.Func, error) {
	if f, ok := ctx.SpecialFuncs[funName]; ok {
		return f, nil
	}
	packageFunName := ctx.PackageData.PackageName + "__" + funName
	if f, ok := ctx.Funcs[packageFunName]; ok {
		return f, nil
	}
	return nil, utils.MakeError("function %s not defined", funName)
}

func (ctx *GenContext) LookupFuncDeclByIR(fun *ir.Func) (*FunctionDecl, error) {
	for name, irf := range ctx.SpecialFuncs {
		if irf == fun {
			return ctx.SpecialFuncDecls[name], nil
		}
	}
	for name, irf := range ctx.Funcs {
		if irf == fun {
			return ctx.PackageData.Functions[name], nil
		}
	}
	return nil, utils.MakeError("function declaration not found for %s", fun.String())
}

func genFunDef(fun *FunctionDecl) (*ir.Func, error) {
	var retType types.Type = types.Void
	var params []*ir.Param
	if len(fun.ReturnTypes) == 1 {
		retType = fun.ReturnTypes[0]
	} else if len(fun.ReturnTypes) > 1 {
		retType = types.Void
		// use func params to return values from function
		for i, p := range fun.ReturnTypes {
			// generate param name if it is not given
			name := fun.ReturnNames[i]
			if name == "" {
				name = fmt.Sprintf("%s__ret_%d", fun.Name, i)
			}
			params = append(params, ir.NewParam(name, types.NewPointer(p)))
		}
	}
	for i, p := range fun.ArgTypes {
		name := fun.ArgNames[i]
		params = append(params, ir.NewParam(name, p))
	}
	return ir.NewFunc(fun.Name, retType, params...), nil
}
