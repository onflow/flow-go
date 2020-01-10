package executor

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	"github.com/dapperlabs/flow-go/model/flow"
)

// A BlockExecutor executes the transactions in a block.
type BlockExecutor interface {
	ExecuteBlock(*ExecutableBlock) (*flow.ExecutionResult, error)
}

type blockExecutor struct {
	vm    virtualmachine.VirtualMachine
	state state.ExecutionState
}

// NewBlockExecutor creates a new block executor.
func NewBlockExecutor(vm virtualmachine.VirtualMachine, state state.ExecutionState) BlockExecutor {
	return &blockExecutor{
		vm:    vm,
		state: state,
	}
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *blockExecutor) ExecuteBlock(
	block *ExecutableBlock,
) (*flow.ExecutionResult, error) {
	chunks, err := e.executeBlock(block)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	// TODO: compute block fees & reward payments

	result := generateExecutionResultForBlock(block, chunks)

	return &result, nil
}

func (e *blockExecutor) executeBlock(
	block *ExecutableBlock,
) ([]*flow.Chunk, error) {
	blockCtx := e.vm.NewBlockContext(block.Block)

	var (
		startState flow.StateCommitment
		err        error
	)

	// get initial start state from parent block
	startState, err = e.state.StateCommitmentByBlockID(block.Block.ParentID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch starting state commitment: %w", err)
	}

	chunks := make([]*flow.Chunk, len(block.Collections))

	for i, collection := range block.Collections {
		chunk, endstate, err := e.executeCollection(i, blockCtx, startState, collection)
		if err != nil {
			return nil, fmt.Errorf("failed to execute collection: %w", err)
		}

		chunks[i] = chunk
		startState = endstate
	}

	return chunks, nil
}

func (e *blockExecutor) executeCollection(
	index int,
	blockCtx virtualmachine.BlockContext,
	startState flow.StateCommitment,
	collection *ExecutableCollection,
) (
	chunk *flow.Chunk,
	endState flow.StateCommitment,
	err error,
) {
	chunkView := e.state.NewView(startState)

	for _, tx := range collection.Transactions {
		txView := chunkView.NewChild()

		result, err := blockCtx.ExecuteTransaction(txView, tx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to execute transaction: %w", err)
		}

		if result.Succeeded() {
			chunkView.ApplyDelta(txView.Delta())
		}
	}

	endState, err = e.state.CommitDelta(chunkView.Delta())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to apply chunk delta: %w", err)
	}

	chunk = &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex: uint(index),
			StartState:      startState,
			// TODO: include event collection hash
			EventCollection: flow.ZeroID,
			// TODO: record gas used
			TotalComputationUsed: 0,
			// TODO: record first tx gas used
			FirstTransactionComputationUsed: 0,
		},
		Index:    0,
		EndState: endState,
	}

	return chunk, endState, nil
}

// generateExecutionResultForBlock creates a new execution result for a block from
// the provided chunk results.
func generateExecutionResultForBlock(block *ExecutableBlock, chunks []*flow.Chunk) flow.ExecutionResult {
	var finalStateCommitment flow.StateCommitment

	// If block is not empty, set final state to the final state of the last chunk.
	// Otherwise, set to the final state of the previous execution result.
	if len(chunks) > 0 {
		finalChunk := chunks[len(chunks)-1]
		finalStateCommitment = finalChunk.EndState
	} else {
		finalStateCommitment = block.PreviousExecutionResult.FinalStateCommitment
	}

	return flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID:     block.PreviousExecutionResult.ID(),
			BlockID:              block.Block.ID(),
			FinalStateCommitment: finalStateCommitment,
			Chunks:               flow.ChunkList{chunks},
		},
	}
}
