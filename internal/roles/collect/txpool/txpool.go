// Package txpool provides a temporary storage utility for pending transactions that
// have yet to be finalized in a block.
package txpool

import (
	"sync"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

// TxPool is a thread-safe in-memory store for pending transactions.
type TxPool struct {
	transactions map[crypto.Hash]types.SignedTransaction
	mutex        sync.RWMutex
}

// New returns a new TxPool.
func New() *TxPool {
	return &TxPool{
		transactions: make(map[crypto.Hash]types.SignedTransaction),
	}
}

// Insert adds a signed transactions to the pool.
func (tp *TxPool) Insert(tx types.SignedTransaction) {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	tp.transactions[tx.Hash()] = tx
}

// Get returns the transaction with the provided hash or nil if it does not exist
// in the pool.
func (tp *TxPool) Get(hash crypto.Hash) types.SignedTransaction {
	return tp.transactions[hash]
}

// Contains returns true if the pool contains a transaction with the provided
// hash, and false otherwise.
func (tp *TxPool) Contains(hash crypto.Hash) bool {
	tp.mutex.RLock()
	defer tp.mutex.RUnlock()

	_, exists := tp.transactions[hash]
	return exists
}

// Remove removes one or more transactions from the pool, specified by hash.
func (tp *TxPool) Remove(hashes ...crypto.Hash) {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	for _, hash := range hashes {
		delete(tp.transactions, hash)
	}
}
