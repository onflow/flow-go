// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Guarantees represents a concurrency-safe memory pool for collection guarantees.
type Guarantees interface {

	// Has checks whether the collection guarantee with the given hash is
	// currently in the memory pool.
	Has(collID flow.Identifier) bool

	// Add will add the given collection guarantee to the memory pool; it will
	// error if the collection guarantee is already in the memory pool.
	Add(guarantee *flow.CollectionGuarantee) error

	// Rem will remove the given collection guarantees from the memory pool; it
	// will return true if the collection guarantees was known and removed.
	Rem(collID flow.Identifier) bool

	// ByID will retrieve the given collection guarantees from the memory pool;
	// it will error if the collection guarantee is not in the memory pool.
	ByID(collID flow.Identifier) (*flow.CollectionGuarantee, error)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all collection guarantees that are currently in the memory pool
	// as a slice.
	All() []*flow.CollectionGuarantee

	// Hash will return a fingerprint has representing the contents of the
	// entire memory pool.
	Hash() flow.Identifier
}
