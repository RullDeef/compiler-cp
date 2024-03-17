package passes

import (
	"gocomp/internal/parser"
	"gocomp/internal/utils"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/value"
)

type deferManager struct {
	deferStack []deferCall
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

func (dm *deferManager) applyDefers(block *ir.Block) {
	for _, dfs := range dm.deferStack {
		block.NewCall(dfs.funRef, dfs.args...)
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
	funName := primExpr2.Operand().OperandName().GetText()
	funRef, err := v.genCtx.LookupFunc(funName)
	if err != nil {
		return nil, err
	}
	args, blocks, err := v.genCtx.GenerateArguments(block, primExpr.Arguments())
	if err != nil {
		return nil, err
	}
	v.deferManager.pushDeferCall(funRef, args)
	return blocks, nil
}
