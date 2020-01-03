package executor

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger"
)

// An BlockExecutor executes the transactions in a block.
type BlockExecutor interface {
	ExecuteBlock(
		block *flow.Block,
		collections []*flow.Collection,
		transactions []*flow.Transaction,
	) ([]*flow.Chunk, error)
}

type blockExecutor struct {
	vm    virtualmachine.VirtualMachine
	state state.ExecutionState
}

// NewBlockExecutor creates a new block executor.
func NewBlockExecutor(vm virtualmachine.VirtualMachine, ls ledger.Storage) BlockExecutor {
	state := state.NewExecutionState(ls)

	return &blockExecutor{
		vm:    vm,
		state: state,
	}
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *blockExecutor) ExecuteBlock(
	block *flow.Block,
	collections []*flow.Collection,
	transactions []*flow.Transaction,
) ([]*flow.Chunk, error) {
	e.vm.SetBlock(block)

	chunks, err := e.executeTransactions(transactions)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	// TODO: compute block fees & reward payments

	return chunks, nil
}

func (e *blockExecutor) executeTransactions(txs []*flow.Transaction) ([]*flow.Chunk, error) {
	// TODO: implement real chunking
	// MVP uses single chunk per block
	chunkView := e.state.NewView(nil)

	for _, tx := range txs {
		txView := chunkView.NewChild()

		result, err := e.vm.ExecuteTransaction(txView, tx)
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

	chunk := &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			FirstTxIndex: 0,
			TxCounts:     uint32(len(txs)),
			// TODO: compute chunk tx collection hash
			ChunkTxCollection: nil,
			// TODO: include start state commitment
			StartState: nil,
			// TODO: include event collection hash
			EventCollection: nil,
			// TODO: record gas used
			TotalComputationUsed: 0,
			// TODO: record first tx gas used
			FirstTransactionComputationUsed: 0,
		},
		Index: 0,
		// TODO: include end state commitment
		EndState: endState,
	}

	return []*flow.Chunk{chunk}, nil
}
