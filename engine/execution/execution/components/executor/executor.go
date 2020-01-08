package executor

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/execution/components/computer"
	"github.com/dapperlabs/flow-go/engine/execution/execution/modules/ledger"
	"github.com/dapperlabs/flow-go/model/flow"
)

// An Executor executes the transactions in a block.
type Executor interface {
	ExecuteBlock(block ExecutableBlock) ([]flow.Chunk, error)
}

type executor struct {
	computer computer.Computer
}

// New creates a new block executor.
func New(computer computer.Computer) Executor {
	return &executor{
		computer: computer,
	}
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *executor) ExecuteBlock(
	block ExecutableBlock,
) ([]flow.Chunk, error) {
	// TODO: validate block, collections and transactions

	chunks, err := e.executeTransactions(block.Transactions)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	return chunks, nil
}

func (e *executor) executeTransactions(txs []flow.TransactionBody) ([]flow.Chunk, error) {
	cache := ledger.NewExecutionCache()

	// TODO: implement real chunking
	// MVP uses single chunk per block
	cache.StartNewChunk()

	for _, tx := range txs {
		view := cache.NewTransactionView()

		result, err := e.computer.ExecuteTransaction(view, tx)
		if err != nil {
			return nil, fmt.Errorf("failed to execute transaction: %w", err)
		}

		if result.Error == nil {
			cache.ApplyTransactionDelta(view.Delta())
		}
	}

	// TODO: commit chunk to storage - https://github.com/dapperlabs/flow-go/issues/1915

	// TODO: implement real chunking
	// MVP uses single chunk per block
	chunk := flow.Chunk{
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
		EndState: nil,
	}

	return []flow.Chunk{chunk}, nil
}
