package passes

import (
	"gocomp/internal/utils"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
)

type GenContext struct {
	PackageData *PackageData

	module       *ir.Module
	Funcs        map[string]*ir.Func
	SpecialFuncs map[string]*ir.Func
	Consts       map[string]*ir.Global

	// global variable context
	Vars *VariableContext
}

func NewGenContext(pdata *PackageData) (*GenContext, error) {
	ctx := GenContext{
		PackageData:  pdata,
		module:       ir.NewModule(),
		Funcs:        make(map[string]*ir.Func),
		SpecialFuncs: make(map[string]*ir.Func),
		Consts:       make(map[string]*ir.Global),
		Vars:         NewVarContext(nil),
	}

	// populate global functions (like printf)
	fun := ir.NewFunc("printf", types.I32, ir.NewParam("format", types.I8Ptr))
	fun.Sig.Variadic = true
	ctx.SpecialFuncs["printf"] = fun

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

func genFunDef(fun *FunctionDecl) (*ir.Func, error) {
	var retType types.Type = types.Void
	if len(fun.ReturnTypes) == 1 {
		retType = fun.ReturnTypes[0]
	} else if len(fun.ReturnTypes) > 1 {
		retType = types.NewStruct(fun.ReturnTypes...)
	}
	var params []*ir.Param
	for i, p := range fun.ArgTypes {
		//TODO: fix param names
		name := fun.ArgNames[i]
		params = append(params, ir.NewParam(name, p))
	}
	return ir.NewFunc(fun.Name, retType, params...), nil
}
