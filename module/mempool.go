// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// CollectionGuaranteePool represents concurrency-safe memory pool for guaranteed collections.
type CollectionGuaranteePool interface {

	// Has checks whether the collection guarantee with the given hash is
	// currently in the memory pool.
	Has(collID flow.Identifier) bool

	// Add will add the given collection guarantee to the memory pool; it will
	// error if the guaranteed collection is already in the memory pool.
	Add(guarantee *flow.CollectionGuarantee) error

	// Rem will remove the given collection guarantees from the memory pool; it
	// will return true if the collection guarantees was known and removed.
	Rem(collID flow.Identifier) bool

	// Get will retrieve the given collection guarantees from the memory pool;
	// it will error if the guaranteed collection is not in the memory pool.
	Get(collID flow.Identifier) (*flow.CollectionGuarantee, error)

	// Hash will return a fingerprint has representing the contents of the
	// entire memory pool.
	Hash() crypto.Hash

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all collections that are currently in the memory pool
	// as a slice.
	All() []*flow.CollectionGuarantee
}

// TransactionPool represents concurrency-safe memory pool for transactions.
type TransactionPool interface {
	// Has checks whether the transaction with the given hash is currently in
	// the memory pool.
	Has(txID flow.Identifier) bool

	// Add will add the given transaction to the memory pool; it will error if
	// the transaction is already in the memory pool.
	Add(tx *flow.Transaction) error

	// Rem will remove the given transaction from the memory pool; it will
	// will return true if the transaction was known and removed.
	Rem(txID flow.Identifier) bool

	// Get will retrieve the given transaction from the memory pool; it will
	// error if the transaction is not in the memory pool.
	Get(txID flow.Identifier) (*flow.Transaction, error)

	// Hash will return a fingerprint has representing the contents of the
	// entire memory pool.
	Hash() crypto.Hash

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all transactions that are currently in the memory pool
	// as a slice.
	All() []*flow.Transaction
}
