package passes

import (
	"fmt"
	"gocomp/internal/parser"

	"github.com/llir/llvm/ir"
)

func (v *CodeGenVisitor) VisitForStmt(block *ir.Block, ctx parser.IForStmtContext) ([]*ir.Block, error) {
	// single expression aka while loop
	if ctx.Expression() != nil {
		return v.VisitWhileLoop(block, ctx)
	} else if ctx.RangeClause() != nil {
		return nil, fmt.Errorf("range for loop not implemented yet")
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

	// condition (assume expression always exist)
	condBlock := ir.NewBlock(fmt.Sprintf("for.cond.%d", stmtUID))
	newBlocks = append(newBlocks, condBlock)
	block.NewBr(condBlock)
	block = condBlock
	val, blocks, err := v.genCtx.GenerateExpr(block, ctx.ForClause().Expression())
	if err != nil {
		return nil, err
	} else if blocks != nil {
		newBlocks = append(newBlocks, blocks...)
		block = newBlocks[len(newBlocks)-1]
	}
	bbody := ir.NewBlock(fmt.Sprintf("for.body.%d", stmtUID))
	bend := ir.NewBlock(fmt.Sprintf("for.end.%d", stmtUID))
	block.NewCondBr(val.Value, bbody, bend)
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
	bpost := ir.NewBlock(fmt.Sprintf("for.post.%d", stmtUID))
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
	newBlocks := []*ir.Block{condBlock}
	block.NewBr(condBlock)
	block = condBlock
	val, blocks, err := v.genCtx.GenerateExpr(block, ctx.Expression())
	if err != nil {
		return nil, err
	} else if blocks != nil {
		newBlocks = append(newBlocks, blocks...)
		block = newBlocks[len(newBlocks)-1]
	}
	bbody := ir.NewBlock(fmt.Sprintf("while.body.%d", stmtUID))
	bend := ir.NewBlock(fmt.Sprintf("while.end.%d", stmtUID))
	block.NewCondBr(val.Value, bbody, bend)
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
