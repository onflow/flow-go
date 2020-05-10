package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification/tracker"
)

// CollectionTrackers represents a concurrency-safe memory pool of collection trackers
type CollectionTrackers interface {

	// Has checks if the given collection ID has a tracker in mempool.
	Has(collID flow.Identifier) bool

	// Add will add the given collection tracker to the memory pool. It will
	// return false if it was already in the mempool.
	Add(collt *tracker.CollectionTracker) bool

	// Rem removes tracker with the given collection ID.
	Rem(collID flow.Identifier) bool

	// ByID retrieve the collection tracker with the given collection ID from
	// the memory pool. It will return false if it was not found in the mempool.
	ByCollectionID(collID flow.Identifier) (*tracker.CollectionTracker, bool)

	// All will return a list of collection trackers in mempool.
	All() []*tracker.CollectionTracker
}
