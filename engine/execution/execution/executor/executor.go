package executor

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	"github.com/dapperlabs/flow-go/model/flow"
)

// A BlockExecutor executes the transactions in a block.
type BlockExecutor interface {
	ExecuteBlock(block ExecutableBlock) (flow.ExecutionResult, error)
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
	block ExecutableBlock,
) (flow.ExecutionResult, error) {
	chunks, err := e.executeTransactions(block.Block, block.Transactions)
	if err != nil {
		return flow.ExecutionResult{}, fmt.Errorf("failed to execute transactions: %w", err)
	}

	// TODO: compute block fees & reward payments

	result := generateExecutionResultForBlock(block, chunks)

	return result, nil
}

func (e *blockExecutor) executeTransactions(
	block flow.Block,
	txs []flow.TransactionBody,
) ([]flow.Chunk, error) {
	blockContext := e.vm.NewBlockContext(&block)

	startState, err := e.state.StateCommitmentByBlockHash(block.Parent)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch starting state commitment: %w", err)
	}

	chunkView := e.state.NewView(startState)

	for _, tx := range txs {
		txView := chunkView.NewChild()

		result, err := blockContext.ExecuteTransaction(txView, tx)
		if err != nil {
			return nil, fmt.Errorf("failed to execute transaction: %w", err)
		}

		if result.Succeeded() {
			chunkView.ApplyDelta(txView.Delta())
		}
	}

	endState, err := e.state.CommitDelta(chunkView.Delta())
	if err != nil {
		return nil, fmt.Errorf("failed to apply chunk delta: %w", err)
	}

	// TODO: (post-MVP) implement real chunking
	// MVP uses single chunk per block
	chunk := flow.Chunk{
		ChunkBody: flow.ChunkBody{
			FirstTxIndex: 0,
			TxCounts:     uint32(len(txs)),
			// TODO: compute chunk tx collection hash
			ChunkTxCollection: nil,
			StartState:        startState,
			// TODO: include event collection hash
			EventCollection: nil,
			// TODO: record gas used
			TotalComputationUsed: 0,
			// TODO: record first tx gas used
			FirstTransactionComputationUsed: 0,
		},
		Index:    0,
		EndState: endState,
	}

	return []flow.Chunk{chunk}, nil
}

// generateExecutionResultForBlock creates a new execution result for a block from
// the provided chunk results.
func generateExecutionResultForBlock(block ExecutableBlock, chunks []flow.Chunk) flow.ExecutionResult {
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
			PreviousExecutionResult: block.PreviousExecutionResult.Fingerprint(),
			Block:                   flow.Fingerprint(block.Block.Hash()),
			FinalStateCommitment:    finalStateCommitment,
			Chunks: flow.ChunkList{
				Chunks: chunks,
			},
		},
	}
}
