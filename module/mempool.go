// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/collection"
)

// Mempool represents concurrency-safe memory pool for guaranteed collections.
type Mempool interface {

	// Has checks whether the guaranteed collection with the given hash is
	// currently in the memory pool.
	Has(hash crypto.Hash) bool

	// Add will add the given guaranteed collection to the memory pool; it will
	// error if the guaranteed collection is already in the memory pool.
	Add(coll *collection.GuaranteedCollection) error

	// Rem will remove the given guaranteed collection from the memory pool; it
	// will return true if the guaranteed collection was known and removed.
	Rem(hash crypto.Hash) bool

	// Get will retrieve the given guaranteed collection from the memory pool;
	// it will error if the guaranteed collection is not in the memory pool.
	Get(hash crypto.Hash) (*collection.GuaranteedCollection, error)

	// Hash will return a fingerprint has representing the contents of the
	// entire memory pool.
	Hash() crypto.Hash

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all collections that are currently in the memory pool
	// as a slice.
	All() []*collection.GuaranteedCollection
}
