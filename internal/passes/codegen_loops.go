package passes

import (
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/utils"

	"github.com/llir/llvm/ir"
)

type loopBlocks struct {
	cond *ir.Block
	end  *ir.Block
}

func (v *CodeGenVisitor) pushLoopStack(cond, end *ir.Block) {
	v.loopStack = append(v.loopStack, loopBlocks{cond, end})
}

func (v *CodeGenVisitor) popLoopStack() {
	v.loopStack = v.loopStack[:len(v.loopStack)-1]
}

func (v *CodeGenVisitor) topLoopBlocks() loopBlocks {
	return v.loopStack[len(v.loopStack)-1]
}

func (v *CodeGenVisitor) VisitForStmt(block *ir.Block, ctx parser.IForStmtContext) ([]*ir.Block, error) {
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
	stmtUID := v.UID
	v.UID++

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
	stmtUID := v.UID
	v.UID++

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
	stmtUID := v.UID
	v.UID++

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
