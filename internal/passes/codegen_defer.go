package passes

import (
	"gocomp/internal/parser"
	"gocomp/internal/typesystem"
	"gocomp/internal/utils"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

const deferSPName = ".deferSP"
const deferStackName = ".deferStack"
const deferStackLen = 1024

type deferManager struct {
	deferStack []deferCall
	stackDef   *ir.Global
	stackSPDef *ir.Global
}

type deferCall struct {
	funRef *ir.Func
	args   []value.Value
}

func (dm *deferManager) clearDeferStack() {
	dm.deferStack = nil
}

func (dm *deferManager) pushDeferCall(funRef *ir.Func, args []value.Value) {
	dm.deferStack = append([]deferCall{{
		funRef: funRef,
		args:   args,
	}}, dm.deferStack...)
}

// must be called for main__init func
func (dm *deferManager) initDeferStack(module *ir.Module, block *ir.Block) {
	tp := types.NewArray(deferStackLen, types.I8)
	dm.stackDef = module.NewGlobal(deferStackName, tp)
	dm.stackDef.Init = constant.NewZeroInitializer(tp)

	tp2 := types.NewPointer(types.I8)
	dm.stackSPDef = module.NewGlobal(deferSPName, tp2)
	dm.stackSPDef.Init = constant.NewZeroInitializer(tp2)

	block.NewStore(typesystem.NewTypedValue(dm.stackDef, tp2), dm.stackSPDef)
}

func (dm *deferManager) applyDefers(block *ir.Block) {
	for _, dfs := range dm.deferStack {
		block.NewCall(dfs.funRef, dfs.args...)
		// block.NewI
	}
}

func (v *CodeGenVisitor) VisitDeferStmt(block *ir.Block, ctx parser.IDeferStmtContext) ([]*ir.Block, error) {
	// defer statement can only be function or method call
	// so we expect ctx to be primary expression
	if ctx.Expression() == nil {
		return nil, utils.MakeError("defer statement must be expression")
	}
	primExpr := ctx.Expression().PrimaryExpr()
	if primExpr == nil {
		return nil, utils.MakeError("defer statement must be primary expression")
	}
	primExpr2 := primExpr.PrimaryExpr()
	if primExpr2 == nil {
		return nil, utils.MakeError("defer statement must be function or method call")
	}
	exprs, blocks, err := v.genCtx.GeneratePrimaryExpr(block, primExpr2)
	if err != nil {
		return nil, err
	} else if blocks != nil {
		block = blocks[len(blocks)-1]
	}
	args, blocks, err := v.genCtx.GenerateArguments(block, primExpr.Arguments())
	if err != nil {
		return nil, err
	}
	v.deferManager.pushDeferCall(exprs[0].(*ir.Func), args)
	return blocks, nil
}
