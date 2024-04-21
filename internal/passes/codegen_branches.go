package passes

import (
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/typesystem"
	"gocomp/internal/utils"

	"github.com/llir/llvm/ir"
)

type branchManager struct {
	// unique index per function body generation
	// used in branch statements
	UID int

	loopStack []loopBlocks
}

type loopBlocks struct {
	cond *ir.Block
	end  *ir.Block
}

func (m *branchManager) EnterFuncDef() {
	m.UID = 0
}

func (m *branchManager) pushLoopStack(cond, end *ir.Block) {
	m.loopStack = append(m.loopStack, loopBlocks{cond, end})
}

func (m *branchManager) popLoopStack() {
	m.loopStack = m.loopStack[:len(m.loopStack)-1]
}

func (m *branchManager) topLoopBlocks() loopBlocks {
	return m.loopStack[len(m.loopStack)-1]
}

func (v *CodeGenVisitor) VisitIfStmt(block *ir.Block, ctx parser.IIfStmtContext) ([]*ir.Block, error) {
	if ctx.SimpleStmt() != nil {
		return nil, utils.MakeError("unsupported init statement in if")
	}
	exprs, newBlocks, err := v.genCtx.GenerateExpr(block, ctx.Expression())
	if err != nil {
		return nil, utils.MakeError("failed to parse if expression: %w", err)
	} else if !typesystem.IsBoolType(exprs[0].Type()) {
		return nil, utils.MakeError("expression must have boolean type")
	} else if newBlocks != nil {
		block = newBlocks[len(newBlocks)-1]
	}
	stmtUID := v.branchManager.UID
	v.branchManager.UID++
	btrue := ir.NewBlock(fmt.Sprintf("btrue.%d", stmtUID))
	bfalse := ir.NewBlock(fmt.Sprintf("bfalse.%d", stmtUID))
	block.NewCondBr(exprs[0], btrue, bfalse)

	newBlocks = append(newBlocks, btrue)
	trueBlocks, err := v.VisitBlock(btrue, ctx.Block(0))
	if err != nil {
		return nil, err
	} else if trueBlocks != nil {
		newBlocks = append(newBlocks, trueBlocks...)
		btrue = newBlocks[len(newBlocks)-1]
	}

	newBlocks = append(newBlocks, bfalse)
	if ctx.ELSE() != nil {
		var blocks []*ir.Block
		var err error
		if ctx.IfStmt() != nil {
			blocks, err = v.VisitIfStmt(bfalse, ctx.IfStmt())
		} else {
			blocks, err = v.VisitBlock(bfalse, ctx.Block(1))
		}
		if err != nil {
			return nil, err
		} else if blocks != nil {
			newBlocks = append(newBlocks, blocks...)
			bfalse = newBlocks[len(newBlocks)-1]
		}
	}

	// end block
	if btrue.Term == nil || bfalse.Term == nil {
		bend := ir.NewBlock(fmt.Sprintf("bend.%d", stmtUID))
		if btrue.Term == nil {
			btrue.NewBr(bend)
		}
		if bfalse.Term == nil {
			bfalse.NewBr(bend)
		}
		newBlocks = append(newBlocks, bend)
	}

	return newBlocks, nil
}

func (v *CodeGenVisitor) VisitForStmt(block *ir.Block, ctx parser.IForStmtContext) ([]*ir.Block, error) {
	v.genCtx.PushLexicalScope()
	defer v.genCtx.PopLexicalScope()
	// single expression aka while loop
	if ctx.Expression() != nil {
		return v.VisitWhileLoop(block, ctx)
	} else if ctx.RangeClause() != nil {
		return nil, utils.MakeError("range for loop not implemented yet")
	} else if ctx.ForClause() != nil {
		return v.VisitForClaused(block, ctx)
	} else {
		// endless loop here
		return v.VisitEndlessLoop(block, ctx)
	}
}

func (v *CodeGenVisitor) VisitEndlessLoop(block *ir.Block, ctx parser.IForStmtContext) ([]*ir.Block, error) {
	stmtUID := v.branchManager.UID
	v.branchManager.UID++

	uroboros := ir.NewBlock(fmt.Sprintf("uroboros.%d", stmtUID))
	bend := ir.NewBlock(fmt.Sprintf("uroboros.end.%d", stmtUID))
	v.pushLoopStack(uroboros, bend)
	defer v.popLoopStack()

	block.NewBr(uroboros)
	block = uroboros
	newBlocks := []*ir.Block{uroboros}
	blocks, err := v.VisitBlock(block, ctx.Block())
	if err != nil {
		return nil, err
	} else if blocks != nil {
		newBlocks = append(newBlocks, blocks...)
		block = newBlocks[len(newBlocks)-1]
	}
	if block.Term == nil {
		block.NewBr(uroboros)
	}
	newBlocks = append(newBlocks, bend)
	return newBlocks, nil
}

func (v *CodeGenVisitor) VisitForClaused(block *ir.Block, ctx parser.IForStmtContext) ([]*ir.Block, error) {
	stmtUID := v.branchManager.UID
	v.branchManager.UID++

	var newBlocks []*ir.Block

	// initialization
	if ctx.ForClause().GetInitStmt() != nil {
		blocks, err := v.VisitSimpleStatement(block, ctx.ForClause().GetInitStmt())
		if err != nil {
			return nil, err
		} else if blocks != nil {
			newBlocks = append(newBlocks, blocks...)
			block = newBlocks[len(newBlocks)-1]
		}
	}

	condBlock := ir.NewBlock(fmt.Sprintf("for.cond.%d", stmtUID))
	bbody := ir.NewBlock(fmt.Sprintf("for.body.%d", stmtUID))
	bpost := ir.NewBlock(fmt.Sprintf("for.post.%d", stmtUID))
	bend := ir.NewBlock(fmt.Sprintf("for.end.%d", stmtUID))
	v.pushLoopStack(bpost, bend)
	defer v.popLoopStack()

	// condition (assume expression always exist)
	newBlocks = append(newBlocks, condBlock)
	block.NewBr(condBlock)
	block = condBlock
	vals, blocks, err := v.genCtx.GenerateExpr(block, ctx.ForClause().Expression())
	if err != nil {
		return nil, err
	} else if blocks != nil {
		newBlocks = append(newBlocks, blocks...)
		block = newBlocks[len(newBlocks)-1]
	}
	block.NewCondBr(vals[0], bbody, bend)
	newBlocks = append(newBlocks, bbody)
	block = bbody

	// loop body
	blocks, err = v.VisitBlock(block, ctx.Block())
	if err != nil {
		return nil, err
	} else if blocks != nil {
		newBlocks = append(newBlocks, blocks...)
		block = newBlocks[len(newBlocks)-1]
	}
	if block.Term == nil {
		block.NewBr(bpost)
	}
	newBlocks = append(newBlocks, bpost)
	block = bpost

	// post condition
	blocks, err = v.VisitSimpleStatement(block, ctx.ForClause().GetPostStmt())
	if err != nil {
		return nil, err
	} else if blocks != nil {
		newBlocks = append(newBlocks, blocks...)
		block = newBlocks[len(newBlocks)-1]
	}
	if block.Term == nil {
		block.NewBr(condBlock)
	}

	newBlocks = append(newBlocks, bend)
	return newBlocks, nil
}

func (v *CodeGenVisitor) VisitWhileLoop(block *ir.Block, ctx parser.IForStmtContext) ([]*ir.Block, error) {
	stmtUID := v.branchManager.UID
	v.branchManager.UID++

	condBlock := ir.NewBlock(fmt.Sprintf("while.cond.%d", stmtUID))
	bbody := ir.NewBlock(fmt.Sprintf("while.body.%d", stmtUID))
	bend := ir.NewBlock(fmt.Sprintf("while.end.%d", stmtUID))
	v.pushLoopStack(condBlock, bend)
	defer v.popLoopStack()

	newBlocks := []*ir.Block{condBlock}
	block.NewBr(condBlock)
	block = condBlock
	vals, blocks, err := v.genCtx.GenerateExpr(block, ctx.Expression())
	if err != nil {
		return nil, err
	} else if blocks != nil {
		newBlocks = append(newBlocks, blocks...)
		block = newBlocks[len(newBlocks)-1]
	}
	block.NewCondBr(vals[0], bbody, bend)
	newBlocks = append(newBlocks, bbody)
	block = bbody
	blocks, err = v.VisitBlock(block, ctx.Block())
	if err != nil {
		return nil, err
	} else if blocks != nil {
		newBlocks = append(newBlocks, blocks...)
		block = newBlocks[len(newBlocks)-1]
	}
	if block.Term == nil {
		block.NewBr(condBlock)
	}
	newBlocks = append(newBlocks, bend)
	return newBlocks, nil
}

func (v *CodeGenVisitor) VisitBreakStmt(block *ir.Block, ctx parser.IBreakStmtContext) error {
	block.NewBr(v.topLoopBlocks().end)
	return nil
}

func (v *CodeGenVisitor) VisitContinueStmt(block *ir.Block, ctx parser.IContinueStmtContext) error {
	block.NewBr(v.topLoopBlocks().cond)
	return nil
}
