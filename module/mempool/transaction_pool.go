// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// TransactionPool implements the transaction memory pool of the collection nodes, used to
// store pending transactions and to generate guaranteed collections.
type TransactionPool struct {
	*Mempool
}

// NewTransactionPool creates a new memory pool for transactions.
func NewTransactionPool() (*TransactionPool, error) {
	inner, err := NewMempool()
	if err != nil {
		return nil, err
	}

	m := &TransactionPool{inner}

	return m, nil
}

// Add adds a transaction to the mempool.
func (m *TransactionPool) Add(tx *flow.Transaction) error {
	return m.Mempool.Add(tx)
}

// Get returns the given transaction from the pool.
func (m *TransactionPool) Get(hash crypto.Hash) (*flow.Transaction, error) {
	item, err := m.Mempool.Get(hash)
	if err != nil {
		return nil, err
	}

	tx, ok := item.(*flow.Transaction)
	if !ok {
		return nil, fmt.Errorf("unable to convert item to transaction")
	}

	return tx, nil
}

// All returns all transactions from the pool.
func (m *TransactionPool) All() []*flow.Transaction {
	items := m.Mempool.All()

	transactions := make([]*flow.Transaction, len(items))

	for i, item := range items {
		transactions[i] = item.(*flow.Transaction)
	}

	return transactions
}
