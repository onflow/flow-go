// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Collections represents a concurrency-safe memory pool for collections.
type Collections interface {

	// Has checks whether the collection with the given hash is currently in
	// the memory pool.
	Has(collID flow.Identifier) bool

	// Add will add the given collection to the memory pool; it will error if
	// the collection is already in the memory pool.
	Add(coll *flow.Collection) error

	// Rem will remove the given collection from the memory pool; it will
	// return true if the collection was known and removed.
	Rem(collID flow.Identifier) bool

	// Get will retrieve the given collection from the memory pool; it will
	// error if the collection is not in the memory pool.
	ByID(collID flow.Identifier) (*flow.Collection, error)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all collections that are currently in the memory pool
	// as a slice.
	All() []*flow.Collection

	// Hash will return a hash of the contents of the memory pool.
	Hash() flow.Identifier
}
