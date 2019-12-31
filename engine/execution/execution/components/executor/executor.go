package executor

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/execution/components/computer"
	"github.com/dapperlabs/flow-go/model/flow"
)

// An Executor executes the transactions in a block.
type Executor interface {
	ExecuteBlock(
		block *flow.Block,
		collections []*flow.Collection,
		transactions []*flow.Transaction,
	) ([]*flow.Chunk, error)
}

type executor struct {
	computer computer.Computer
}

// New creates a new  block executor.
func New(computer computer.Computer) Executor {
	return &executor{
		computer: computer,
	}
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *executor) ExecuteBlock(
	block *flow.Block,
	collections []*flow.Collection,
	transactions []*flow.Transaction,
) ([]*flow.Chunk, error) {
	// TODO: validate block, collections and transactions

	chunks, err := e.executeTransactions(transactions)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	return chunks, nil
}

func (e *executor) executeTransactions(txs []*flow.Transaction) ([]*flow.Chunk, error) {
	results := make([]*computer.TransactionResult, len(txs))

	for i, tx := range txs {
		// TODO: connect ledger to chunk cache - https://github.com/dapperlabs/flow-go/issues/1914
		ledger := flow.NewLedgerView(func(key string) ([]byte, error) { return nil, nil })

		result, err := e.computer.ExecuteTransaction(ledger, tx)
		if err != nil {
			return nil, fmt.Errorf("failed to execute transaction: %w", err)
		}

		results[i] = result
	}

	// TODO: commit chunk to storage - https://github.com/dapperlabs/flow-go/issues/1915

	// TODO: implement real chunking
	// MVP uses single chunk per block
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
		EndState: nil,
	}

	return []*flow.Chunk{chunk}, nil
}
