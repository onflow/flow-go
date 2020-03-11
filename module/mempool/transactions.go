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

	// Add will add the given transaction to the memory pool; it will error if
	// the transaction is already in the memory pool.
	Add(tx *flow.TransactionBody) error

	// Rem will remove the given transaction from the memory pool; it will
	// will return true if the transaction was known and removed.
	Rem(txID flow.Identifier) bool

	// ByID will retrieve the given transaction from the memory pool; it will
	// error if the transaction is not in the memory pool.
	ByID(txID flow.Identifier) (*flow.TransactionBody, error)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all transactions that are currently in the memory pool
	// as a slice.
	All() []*flow.TransactionBody

	// Hash will return a fingerprint has representing the contents of the
	// entire memory pool.
	Hash() flow.Identifier
}
