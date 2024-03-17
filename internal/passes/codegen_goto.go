package passes

import (
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/utils"

	"github.com/llir/llvm/ir"
)

type labelManager struct {
	labels map[string]*smartLabel
}

type smartLabel struct {
	block   *ir.Block
	forward bool
}

func (lm *labelManager) clearLabels() {
	lm.labels = make(map[string]*smartLabel)
}

func (lm *labelManager) checkLabelsDefined() error {
	for label, def := range lm.labels {
		if def.forward {
			return utils.MakeError("label %s not defined", label)
		}
	}
	return nil
}

func (lm *labelManager) addLabel(label string, block *ir.Block) (*ir.Block, error) {
	if sl, ok := lm.labels[label]; ok {
		if sl.forward {
			sl.forward = false
			sl.block = ir.NewBlock(fmt.Sprintf("label.forward.%s", label))
			block.NewBr(sl.block)
			return sl.block, nil
		} else {
			return nil, utils.MakeError("label %s already defined", label)
		}
	}
	newBlock := ir.NewBlock(fmt.Sprintf("label.%s", label))
	block.NewBr(newBlock)
	lm.labels[label] = &smartLabel{
		block:   newBlock,
		forward: false,
	}
	return newBlock, nil
}

func (lm *labelManager) GetLabel(label string) (*ir.Block, error) {
	if sl, ok := lm.labels[label]; !ok {
		block := ir.NewBlock(fmt.Sprintf("label.forward.%s", label))
		lm.labels[label] = &smartLabel{
			block:   block,
			forward: true,
		}
		return block, nil
	} else {
		return sl.block, nil
	}
}

func (v *CodeGenVisitor) VisitLabeledStmt(block *ir.Block, ctx parser.ILabeledStmtContext) ([]*ir.Block, error) {
	block, err := v.addLabel(ctx.IDENTIFIER().GetText(), block)
	if err != nil {
		return nil, err
	}
	newBlocks, err := v.VisitStatement(block, ctx.Statement())
	if err != nil {
		return nil, err
	}
	newBlocks = append([]*ir.Block{block}, newBlocks...)
	return newBlocks, nil
}

func (v *CodeGenVisitor) VisitGotoStmt(block *ir.Block, ctx parser.IGotoStmtContext) ([]*ir.Block, error) {
	labelBlock, err := v.GetLabel(ctx.IDENTIFIER().GetText())
	if err != nil {
		return nil, err
	}
	block.NewBr(labelBlock)
	return []*ir.Block{ir.NewBlock("")}, nil
}
