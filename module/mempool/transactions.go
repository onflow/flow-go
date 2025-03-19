package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// Transactions represents a concurrency-safe memory pool for transactions.
type Transactions interface {

	// Has checks whether the transaction with the given hash is currently in
	// the memory pool.
	Has(txID flow.Identifier) bool

	// Add will add the given transaction body to the memory pool. It will
	// return false if it was already in the mempool.
	Add(txId flow.Identifier, tx *flow.TransactionBody) bool

	// Remove will remove the given transaction from the memory pool; it will
	// return true if the transaction was known and removed.
	Remove(txID flow.Identifier) bool

	// Get retrieve the transaction with the given ID from the memory
	// pool. It will return false if it was not found in the mempool.
	Get(txID flow.Identifier) (*flow.TransactionBody, bool)

	// Size will return the current size of the memory pool.
	Size() uint

	// Values will retrieve all transactions that are currently in the memory pool
	// as a slice.
	Values() []*flow.TransactionBody

	// Clear removes all transactions from the mempool.
	Clear()
}
