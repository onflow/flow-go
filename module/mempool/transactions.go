// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Transactions represents a concurrency-safe memory pool for transactions.
type Transactions interface {

	// Has checks whether the transaction with the given hash is currently in
	// the memory pool.
	Has(txID flow.Identifier) bool

	// Add will add the given transaction body to the memory pool. It will
	// return false if it was already in the mempool.
	Add(tx *flow.TransactionBody) bool

	// Rem will remove the given transaction from the memory pool; it will
	// will return true if the transaction was known and removed.
	Rem(txID flow.Identifier) bool

	// ByID retrieve the transaction with the given ID from the memory
	// pool. It will return false if it was not found in the mempool.
	ByID(txID flow.Identifier) (*flow.TransactionBody, bool)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all transactions that are currently in the memory pool
	// as a slice.
	All() []*flow.TransactionBody

	// Hash will return a fingerprint has representing the contents of the
	// entire memory pool.
	Hash() flow.Identifier
}
