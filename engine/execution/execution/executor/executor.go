package executor

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	"github.com/dapperlabs/flow-go/model/flow"
)

// A BlockExecutor executes the transactions in a block.
type BlockExecutor interface {
	ExecuteBlock(*execution.CompleteBlock, *state.View, flow.StateCommitment) (*execution.ComputationResult, error)
}

type blockExecutor struct {
	vm virtualmachine.VirtualMachine
}

// NewBlockExecutor creates a new block executor.
func NewBlockExecutor(vm virtualmachine.VirtualMachine) BlockExecutor {
	return &blockExecutor{
		vm: vm,
	}
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *blockExecutor) ExecuteBlock(
	block *execution.CompleteBlock,
	view *state.View,
	startState flow.StateCommitment,
) (*execution.ComputationResult, error) {
	results, err := e.executeBlock(block, view, startState)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	// TODO: compute block fees & reward payments

	return results, nil
}

func (e *blockExecutor) executeBlock(
	block *execution.CompleteBlock,
	view *state.View,
	startState flow.StateCommitment,
) (*execution.ComputationResult, error) {

	blockCtx := e.vm.NewBlockContext(&block.Block.Header)

	collections := block.Collections()

	views := make([]*state.View, len(collections))

	for i, collection := range collections {

		collectionView := view.NewChild()

		err := e.executeCollection(i, blockCtx, collectionView, collection)
		if err != nil {
			return nil, fmt.Errorf("failed to execute collection: %w", err)
		}

		views[i] = collectionView

		view.ApplyDelta(collectionView.Delta())
	}

	return &execution.ComputationResult{
		CompleteBlock: block,
		StateViews:    views,
		StartState:    startState,
	}, nil
}

func (e *blockExecutor) executeCollection(
	index int,
	blockCtx virtualmachine.BlockContext,
	chunkView *state.View,
	collection *execution.CompleteCollection,
) error {

	for _, tx := range collection.Transactions {
		txView := chunkView.NewChild()

		result, err := blockCtx.ExecuteTransaction(txView, tx)
		if err != nil {
			return fmt.Errorf("failed to execute transaction: %w", err)
		}

		if result.Succeeded() {
			chunkView.ApplyDelta(txView.Delta())
		}
	}

	return nil
}
