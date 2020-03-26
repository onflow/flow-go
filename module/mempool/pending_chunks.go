// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
)

// PendingCollections represents a concurrency-safe memory pool for pending collections.
type PendingCollections interface {
	// Has checks whether the pending collection with the given ID is currently in
	// the memory pool.
	Has(pcollID flow.Identifier) bool

	// Add will add the given pending collection to the memory pool; it will error if
	// the collection is already in the memory pool.
	Add(pcoll *verification.PendingCollection) error

	// Rem will remove the given pending collection from the memory pool; it will
	// return true if the collection was known and removed.
	Rem(pcollID flow.Identifier) bool

	// ByID will retrieve the given collection from the memory pool; it will
	// error if the collection is not in the memory pool.
	ByID(pcollID flow.Identifier) (*verification.PendingCollection, error)

	// All will retrieve all pending collections that are currently in the memory pool
	// as a slice.
	All() []*flow.Collection
}
