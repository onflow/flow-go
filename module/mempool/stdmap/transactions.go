// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// Transactions implements the transactions memory pool of the consensus nodes,
// used to store transactions and to generate block payloads.
type Transactions struct {
	*Backend
}

// NewTransactions creates a new memory pool for transactions.
// Deprecated: use herocache.Transactions instead.
func NewTransactions(limit uint) *Transactions {
	t := &Transactions{
		Backend: NewBackend(WithLimit(limit)),
	}

	return t
}

// Add adds a transaction to the mempool.
func (t *Transactions) Add(tx *flow.TransactionBody) bool {
	return t.Backend.Add(tx)
}

// ByID returns the transaction with the given ID from the mempool.
func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, bool) {
	entity, exists := t.Backend.ByID(txID)
	if !exists {
		return nil, false
	}
	tx, ok := entity.(*flow.TransactionBody)
	if !ok {
		panic(fmt.Sprintf("invalid entity in transaction pool (%T)", entity))
	}
	return tx, true
}

// All returns all transactions from the mempool.
func (t *Transactions) All() []*flow.TransactionBody {
	entities := t.Backend.All()
	txs := make([]*flow.TransactionBody, 0, len(entities))
	for _, entity := range entities {
		txs = append(txs, entity.(*flow.TransactionBody))
	}
	return txs
}
