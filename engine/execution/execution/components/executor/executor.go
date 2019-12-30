package executor

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/execution/components/computer"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

// An Executor executes the transactions in a block.
type Executor struct {
	collections  storage.Collections
	transactions storage.Transactions
	computer     computer.Computer
}

// New creates a new  block executor.
func New(cols storage.Collections, txs storage.Transactions, computer computer.Computer) *Executor {
	return &Executor{
		collections:  cols,
		transactions: txs,
		computer:     computer,
	}
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *Executor) ExecuteBlock(
	block *flow.Block,
) ([]*flow.Chunk, error) {
	// TODO: validate block, collections and transactions

	collections, err := e.getCollections(block.GuaranteedCollections)
	if err != nil {
		return nil, fmt.Errorf("failed to load collections: %w", err)
	}

	transactions, err := e.getTransactions(collections)
	if err != nil {
		return nil, fmt.Errorf("failed to load transactions: %w", err)
	}

	chunks, err := e.executeTransactions(transactions)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	return chunks, nil
}

func (e *Executor) getCollections(cols []*flow.GuaranteedCollection) ([]*flow.Collection, error) {
	collections := make([]*flow.Collection, len(cols))

	for i, gc := range cols {
		c, err := e.collections.ByFingerprint(gc.Fingerprint())
		if err != nil {
			return nil, fmt.Errorf("failed to load collection: %w", err)
		}

		collections[i] = c
	}

	return collections, nil
}

func (e *Executor) getTransactions(cols []*flow.Collection) ([]*flow.Transaction, error) {
	txCount := 0

	for _, c := range cols {
		txCount += c.Size()
	}

	transactions := make([]*flow.Transaction, txCount)

	i := 0

	for _, c := range cols {
		for _, f := range c.Transactions {
			tx, err := e.transactions.ByFingerprint(f)
			if err != nil {
				return nil, fmt.Errorf("failed to load transaction: %w", err)
			}

			transactions[i] = tx
			i++
		}
	}

	return transactions, nil
}

func (e *Executor) executeTransactions(txs []*flow.Transaction) ([]*flow.Chunk, error) {
	results := make([]*computer.TransactionResult, len(txs))

	for i, tx := range txs {
		result, err := e.computer.ExecuteTransaction(tx)
		if err != nil {
			return nil, fmt.Errorf("failed to execute transaction: %w", err)
		}

		results[i] = result
	}

	// TODO: for each chunk, store results and generate a state proof

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
