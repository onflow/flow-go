// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Transactions implements the transactions memory pool of the consensus nodes,
// used to store transactions and to generate block payloads.
type Transactions struct {
	*Backend
}

// NewTransactions creates a new memory pool for transactions.
func NewTransactions() (*Transactions, error) {
	t := &Transactions{
		Backend: NewBackend(),
	}

	return t, nil
}

// Add adds a transaction to the mempool.
func (t *Transactions) Add(tx *flow.Transaction) error {
	return t.Backend.Add(tx)
}

// Get returns the transaction with the given ID from the mempool.
func (t *Transactions) Get(txID flow.Identifier) (*flow.Transaction, error) {
	entity, err := t.Backend.Get(txID)
	if err != nil {
		return nil, err
	}
	tx, ok := entity.(*flow.Transaction)
	if !ok {
		panic(fmt.Sprintf("invalid entity in transaction pool (%T)", entity))
	}
	return tx, nil
}

// All returns all transactions from the mempool.
func (t *Transactions) All() []*flow.Transaction {
	entities := t.Backend.All()
	txs := make([]*flow.Transaction, 0, len(entities))
	for _, entity := range entities {
		tx, ok := entity.(*flow.Transaction)
		if !ok {
			panic(fmt.Sprintf("invalid entity in transaction pool (%T)", entity))
		}
		txs = append(txs, tx)
	}
	return txs
}
