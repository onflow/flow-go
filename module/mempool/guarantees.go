package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// Guarantees represents a concurrency-safe memory pool for collection guarantees.
type Guarantees interface {

	// Has checks whether the collection guarantee with the given hash is
	// currently in the memory pool.
	Has(collID flow.Identifier) bool

	// Add will add the given collection guarantee to the memory pool. It will
	// return false if it was already in the mempool.
	Add(guarantee *flow.CollectionGuarantee) bool

	// Remove will remove the given collection guarantees from the memory pool; it
	// will return true if the collection guarantees was known and removed.
	Remove(collID flow.Identifier) bool

	// ByID retrieve the collection guarantee with the given ID from the memory
	// pool. It will return false if it was not found in the mempool.
	ByID(collID flow.Identifier) (*flow.CollectionGuarantee, bool)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all collection guarantees that are currently in the memory pool
	// as a slice.
	All() []*flow.CollectionGuarantee
}
