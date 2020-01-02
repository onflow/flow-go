// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// CollectionPool represents concurrency-safe memory pool for guaranteed collections.
type CollectionPool interface {

	// Has checks whether the guaranteed collection with the given hash is
	// currently in the memory pool.
	Has(fp flow.Fingerprint) bool

	// Add will add the given guaranteed collection to the memory pool; it will
	// error if the guaranteed collection is already in the memory pool.
	Add(coll *flow.GuaranteedCollection) error

	// Rem will remove the given guaranteed collection from the memory pool; it
	// will return true if the guaranteed collection was known and removed.
	Rem(fp flow.Fingerprint) bool

	// Get will retrieve the given guaranteed collection from the memory pool;
	// it will error if the guaranteed collection is not in the memory pool.
	Get(fp flow.Fingerprint) (*flow.GuaranteedCollection, error)

	// Hash will return a fingerprint has representing the contents of the
	// entire memory pool.
	Hash() crypto.Hash

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all collections that are currently in the memory pool
	// as a slice.
	All() []*flow.GuaranteedCollection
}

// TransactionPool represents concurrency-safe memory pool for transactions.
type TransactionPool interface {
	// Has checks whether the transaction with the given hash is currently in
	// the memory pool.
	Has(fp flow.Fingerprint) bool

	// Add will add the given transaction to the memory pool; it will error if
	// the transaction is already in the memory pool.
	Add(tx *flow.Transaction) error

	// Rem will remove the given transaction from the memory pool; it will
	// will return true if the transaction was known and removed.
	Rem(fp flow.Fingerprint) bool

	// Get will retrieve the given transaction from the memory pool; it will
	// error if the transaction is not in the memory pool.
	Get(fp flow.Fingerprint) (*flow.Transaction, error)

	// Hash will return a fingerprint has representing the contents of the
	// entire memory pool.
	Hash() crypto.Hash

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all transactions that are currently in the memory pool
	// as a slice.
	All() []*flow.Transaction
}
