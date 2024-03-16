package passes

import (
	"fmt"

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

func NewGenContext(pdata *PackageData) *GenContext {
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
		irFun := genFunDef(fn)
		irFun.Parent = ctx.module
		ctx.Funcs[fn.Name] = irFun
	}

	return &ctx
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
	return nil, fmt.Errorf("function %s not defined", funName)
}

func genFunDef(fun FunctionDecl) *ir.Func {
	var retType types.Type = types.Void
	if len(fun.ReturnTypes) == 1 {
		var err error
		retType, err = goTypeToIR(fun.ReturnTypes[0].Type)
		if err != nil {
			panic(fmt.Errorf("unimplemented return type: %w", err))
		}
	} else if len(fun.ReturnTypes) > 1 {
		var fields []types.Type
		for _, tp := range fun.ReturnTypes {
			if llvmTp, err := goTypeToIR(tp.Type); err != nil {
				panic(err)
			} else {
				fields = append(fields, llvmTp)
			}
		}
		retType = types.NewStruct(fields...)
	}
	var params []*ir.Param
	for _, p := range fun.ArgTypes {
		t, err := goTypeToIR(p.Type)
		if err != nil {
			panic(fmt.Errorf("unimplemented arg type: %w", err))
		}
		params = append(params, ir.NewParam(p.Name, t))
	}
	return ir.NewFunc(fun.Name, retType, params...)
}

func goTypeToIR(goType string) (types.Type, error) {
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
		return nil, fmt.Errorf("invalid primitive type: %s", goType)
	}
	return t, nil
}
