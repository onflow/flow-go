package epochs

import (
	"sync"

	"github.com/onflow/flow-go/module/mempool"
)

// TransactionPools is a set of epoch-scoped transaction pools. Each pool is a
// singleton that is instantiated the first time a transaction pool for that
// epoch is requested.
//
// This enables decoupled components to share access to the same transaction
// pools across epochs, while maintaining the property that one transaction
// pool is only valid for a single epoch.
type TransactionPools struct {
	mu     sync.RWMutex
	pools  map[uint64]mempool.Transactions
	create func(uint64) mempool.Transactions
}

// NewTransactionPools returns a new set of epoch-scoped transaction pools.
func NewTransactionPools(create func(uint64) mempool.Transactions) *TransactionPools {

	pools := &TransactionPools{
		pools:  make(map[uint64]mempool.Transactions),
		create: create,
	}
	return pools
}

// ForEpoch returns the transaction pool for the given pool. All calls for
// the same epoch will return the same underlying transaction pool.
func (t *TransactionPools) ForEpoch(epoch uint64) mempool.Transactions {

	t.mu.RLock()
	pool, exists := t.pools[epoch]
	t.mu.RUnlock()
	if exists {
		return pool
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	pool = t.create(epoch)
	t.pools[epoch] = pool
	return pool
}

// CombinedSize returns the sum of the sizes of all transaction pools.
func (t *TransactionPools) CombinedSize() uint {

	t.mu.RLock()
	defer t.mu.RUnlock()

	size := uint(0)
	for _, pool := range t.pools {
		size += pool.Size()
	}

	return size
}
