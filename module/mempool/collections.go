package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// Collections represents a concurrency-safe memory pool for collections.
type Collections interface {

	// Has checks whether the collection with the given hash is currently in
	// the memory pool.
	Has(collID flow.Identifier) bool

	// Add will add the given collection to the memory pool. It will return
	// false if it was already in the mempool.
	Add(coll *flow.Collection) bool

	// Remove will remove the given collection from the memory pool; it will
	// return true if the collection was known and removed.
	Remove(collID flow.Identifier) bool

	// ByID retrieve the collection with the given ID from the memory pool.
	// It will return false if it was not found in the mempool.
	ByID(collID flow.Identifier) (*flow.Collection, bool)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all collections that are currently in the memory pool
	// as a slice.
	All() []*flow.Collection
}
