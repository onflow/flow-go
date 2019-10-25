// Package txpool provides a temporary storage utility for pending transactions that
// have yet to be finalized in a block.
package txpool

import (
	"sync"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
)

// TxPool is a thread-safe in-memory store for pending transactions.
type TxPool struct {
	transactions map[string]types.Transaction
	mutex        sync.RWMutex
}

// New returns a new TxPool.
func New() *TxPool {
	return &TxPool{
		transactions: make(map[string]types.Transaction),
	}
}

// Insert adds a signed transactions to the pool.
func (tp *TxPool) Insert(tx types.Transaction) {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	tp.transactions[string(tx.Hash)] = tx
}

// Get returns the transaction with the provided hash or nil if it does not exist
// in the pool.
func (tp *TxPool) Get(hash crypto.Hash) types.Transaction {
	return tp.transactions[string(hash)]
}

// Contains returns true if the pool contains a transaction with the provided
// hash, and false otherwise.
func (tp *TxPool) Contains(hash crypto.Hash) bool {
	tp.mutex.RLock()
	defer tp.mutex.RUnlock()

	_, exists := tp.transactions[string(hash)]
	return exists
}

// Remove removes one or more transactions from the pool, specified by hash.
func (tp *TxPool) Remove(hashes ...crypto.Hash) {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	for _, hash := range hashes {
		delete(tp.transactions, string(hash))
	}
}
