package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// Transactions implements the transactions memory pool of the consensus nodes,
// used to store transactions and to generate block payloads.
type Transactions struct {
	*Backend[flow.Identifier, *flow.TransactionBody]
}

// NewTransactions creates a new memory pool for transactions.
// Deprecated: use herocache.Transactions instead.
func NewTransactions(limit uint) *Transactions {
	t := &Transactions{
		Backend: NewBackend[flow.Identifier, *flow.TransactionBody](WithLimit[flow.Identifier, *flow.TransactionBody](limit)),
	}

	return t
}

// Add adds a transaction to the mempool.
func (t *Transactions) Add(tx *flow.TransactionBody) bool {
	return t.Backend.Add(tx.ID(), tx)
}

// ByID returns the transaction with the given ID from the mempool.
func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, bool) {
	tx, exists := t.Backend.ByID(txID)
	if !exists {
		return nil, false
	}
	return tx, true
}

// All returns all transactions from the mempool.
func (t *Transactions) All() []*flow.TransactionBody {
	entities := t.Backend.All()
	txs := make([]*flow.TransactionBody, 0, len(entities))
	for _, tx := range entities {
		txs = append(txs, tx)
	}
	return txs
}
