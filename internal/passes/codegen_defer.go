package passes

import (
	"fmt"
	"gocomp/internal/parser"
	"gocomp/internal/utils"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type deferManager struct {
	deferStack        []*ir.InstAlloca
	deferCounter      int // how many defer statements encountered so far in current function
	deferApplyCounter int
}

// type used in LLVM IR code to keep track of defered calls
var deferCallStackType = func() *types.StructType {
	s := &types.StructType{
		TypeName: "__deferCall",
		Fields: []types.Type{
			types.NewPointer(types.NewFunc(types.Void, types.I8Ptr)),
			types.I8Ptr,
		},
	}
	// reference to next stack node
	s.Fields = append(s.Fields, &types.PointerType{ElemType: s})
	return s
}()
var dfStackNodePtr = types.NewPointer(deferCallStackType)
var deferCallStackTypeSize = 32 // more than enough

func (dm *deferManager) setupDeferStack(block *ir.Block) {
	callStack := block.NewAlloca(types.NewPointer(deferCallStackType))
	block.NewStore(constant.NewNull(dfStackNodePtr), callStack)
	dm.deferStack = append([]*ir.InstAlloca{callStack}, dm.deferStack...)
	dm.deferCounter = 0
	dm.deferApplyCounter = 0
}

func (dm *deferManager) clearDeferStack() {
	dm.deferStack = dm.deferStack[1:]
}

func (v *CodeGenVisitor) pushDeferCall(block *ir.Block, funRef *ir.Func, args []value.Value) error {
	malloc, err := v.genCtx.LookupFunc("GC_malloc")
	if err != nil {
		return err
	}

	v.deferCounter++

	// function declaration for multiple return values support
	funDecl, err := v.genCtx.LookupFuncDeclByIR(funRef)
	if err != nil {
		return utils.MakeError("function declaration for %s not found", funRef.String())
	}

	// create args struct definition
	module := v.currentFuncIR.Parent
	tpDefName := "__df_" + funRef.Name()
	var tpDef types.Type
	for _, tpd := range module.TypeDefs {
		if tpd.Name() == tpDefName {
			tpDef = tpd
			break
		}
	}
	if tpDef == nil {
		argTypes := []types.Type{}
		for _, arg := range args {
			argTypes = append(argTypes, arg.Type())
		}
		tpDef = types.NewStruct(argTypes...)
		tpDef.SetName(tpDefName)
		module.NewTypeDef(tpDefName, tpDef)
	}

	// create wrapper function
	wrapperFunName := "__df_wrpr_" + funRef.Name()
	var wrapperFun *ir.Func
	for _, fn := range module.Funcs {
		if fn.Name() == wrapperFunName {
			wrapperFun = fn
			break
		}
	}
	if wrapperFun == nil {
		wrapperFun = module.NewFunc(wrapperFunName, types.Void, ir.NewParam("args", types.I8Ptr))
		// fill function body
		entry := wrapperFun.NewBlock("entry")
		if args == nil && len(funDecl.ReturnTypes) <= 1 {
			entry.NewCall(funRef)
		} else {
			argsStruct := entry.NewBitCast(wrapperFun.Params[0], types.NewPointer(tpDef))
			// load real function arguments from passed struct (named args)
			argValues := []value.Value{}
			if len(funDecl.ReturnTypes) > 1 {
				for _, argTp := range funDecl.ReturnTypes {
					mem := entry.NewAlloca(argTp)
					argValues = append(argValues, mem)
				}
			}
			for i, arg := range args {
				argValues = append(argValues, entry.NewLoad(
					arg.Type(),
					entry.NewGetElementPtr(
						tpDef,
						argsStruct,
						constant.NewInt(types.I32, 0),
						constant.NewInt(types.I32, int64(i)),
					),
				))
			}
			entry.NewCall(funRef, argValues...)
		}
		entry.NewRet(nil)
	}

	// create defer stack node
	nodeMem := block.NewBitCast(
		block.NewCall(malloc, constant.NewInt(types.I64, int64(deferCallStackTypeSize))),
		dfStackNodePtr,
	)

	// update its fields
	node_FuncRef := block.NewGetElementPtr(
		deferCallStackType,
		nodeMem,
		constant.NewInt(types.I32, 0),
		constant.NewInt(types.I32, 0),
	)
	block.NewStore(wrapperFun, node_FuncRef)

	if args != nil || funDecl.ReturnTypes != nil {
		node_argsRef := block.NewGetElementPtr(
			deferCallStackType,
			nodeMem,
			constant.NewInt(types.I32, 0),
			constant.NewInt(types.I32, 1),
		)
		// TODO: merge malloc calls
		// TODO: calculate exact storage size needed
		argsStructRaw := block.NewCall(malloc, constant.NewInt(types.I64, 32))
		argsStruct := block.NewBitCast(argsStructRaw, types.NewPointer(tpDef))
		// fill struct fields
		for i, arg := range args {
			offset := block.NewGetElementPtr(
				tpDef,
				argsStruct,
				constant.NewInt(types.I32, 0),
				constant.NewInt(types.I32, int64(i)),
			)
			block.NewStore(arg, offset)
		}
		block.NewStore(argsStructRaw, node_argsRef)
	}

	node_NextRef := block.NewGetElementPtr(
		deferCallStackType,
		nodeMem,
		constant.NewInt(types.I32, 0),
		constant.NewInt(types.I32, 2),
	)
	nextRef := block.NewLoad(dfStackNodePtr, v.deferStack[0])
	block.NewStore(nextRef, node_NextRef)

	// update local stack head
	block.NewStore(nodeMem, v.deferStack[0])
	return nil
}

// must be called from main__init func
func (dm *deferManager) initDeferStack(module *ir.Module, block *ir.Block) {
	module.NewTypeDef("__deferCall", deferCallStackType)
}

// must be called from main__cleanup func
func (dm *deferManager) cleanupDeferStack(module *ir.Module, block *ir.Block) {
	// pass
}

func (dm *CodeGenVisitor) applyDefers(block *ir.Block) []*ir.Block {
	if dm.deferCounter == 0 {
		// nothing to do - no waste of CPU cycles
		return nil
	}

	// traverse defer stack list from head
	loopStart := ir.NewBlock(fmt.Sprintf("__df_loop_start.%d", dm.deferApplyCounter))
	loopBody := ir.NewBlock(fmt.Sprintf("__df_loop_body.%d", dm.deferApplyCounter))
	loopEnd := ir.NewBlock(fmt.Sprintf("__df_loop_end.%d", dm.deferApplyCounter))
	dm.deferApplyCounter++

	block.NewBr(loopStart)

	// check if stack is empty
	nodeRef := loopStart.NewLoad(dfStackNodePtr, dm.deferStack[0])
	cmpRes := loopStart.NewICmp(enum.IPredEQ, nodeRef, constant.NewNull(dfStackNodePtr))
	loopStart.NewCondBr(cmpRes, loopEnd, loopBody)

	// load stack head
	nodeRef = loopBody.NewLoad(dfStackNodePtr, dm.deferStack[0])

	// call defered function
	funcOffset := loopBody.NewGetElementPtr(
		deferCallStackType,
		nodeRef,
		constant.NewInt(types.I32, 0),
		constant.NewInt(types.I32, 0),
	)
	funcRef := loopBody.NewLoad(types.NewPointer(types.NewFunc(types.Void, types.I8Ptr)), funcOffset)

	// load arguments struct
	argsStruct := loopBody.NewLoad(
		types.I8Ptr,
		loopBody.NewGetElementPtr(
			deferCallStackType,
			nodeRef,
			constant.NewInt(types.I32, 0),
			constant.NewInt(types.I32, 1),
		),
	)
	loopBody.NewCall(funcRef, argsStruct)

	// update head to next node
	nextNodeRef := loopBody.NewGetElementPtr(
		deferCallStackType,
		nodeRef,
		constant.NewInt(types.I32, 0),
		constant.NewInt(types.I32, 2),
	)
	nextNode := loopBody.NewLoad(dfStackNodePtr, nextNodeRef)
	loopBody.NewStore(nextNode, dm.deferStack[0])
	// goto loop start
	loopBody.NewBr(loopStart)

	return []*ir.Block{loopStart, loopBody, loopEnd}
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
	err = v.pushDeferCall(block, exprs[0].(*ir.Func), args)
	if err != nil {
		return nil, err
	}
	return blocks, nil
}
