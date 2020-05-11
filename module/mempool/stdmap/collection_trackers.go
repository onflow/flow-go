package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification/tracker"
)

// CollectionTrackers implements the CollectionTrackers memory pool.
type CollectionTrackers struct {
	*Backend
}

// NewCollectionTrackers creates a new memory pool for CollectionTrackers.
func NewCollectionTrackers(limit uint) (*CollectionTrackers, error) {
	a := &CollectionTrackers{
		Backend: NewBackend(WithLimit(limit)),
	}
	return a, nil
}

// Add adds a CollectionTracker to the mempool.
func (c *CollectionTrackers) Add(ct *tracker.CollectionTracker) bool {
	return c.Backend.Add(ct)
}

// Has checks whether the CollectionTracker with the given collection ID is currently in
// the memory pool.
func (c *CollectionTrackers) Has(collID flow.Identifier) bool {
	c.Lock()
	defer c.Unlock()

	return c.Backend.Has(collID)
}

// Rem removes tracker with the given collection Id.
func (c *CollectionTrackers) Rem(collID flow.Identifier) bool {
	return c.Backend.Rem(collID)
}

// Inc atomically increases the counter of tracker by one and returns the updated tracker
func (c *CollectionTrackers) Inc(collID flow.Identifier) (*tracker.CollectionTracker, error) {
	updated, ok := c.Backend.Adjust(collID, func(entity flow.Entity) flow.Entity {
		ct := entity.(*tracker.CollectionTracker)
		return &tracker.CollectionTracker{
			CollectionID: ct.CollectionID,
			BlockID:      ct.BlockID,
			Counter:      ct.Counter + 1,
		}
	})

	if !ok {
		return nil, fmt.Errorf("could not update tracker in backend")
	}

	return updated.(*tracker.CollectionTracker), nil
}

// ByCollectionID returns the collection tracker for the given collection ID.
func (c *CollectionTrackers) ByCollectionID(collID flow.Identifier) (*tracker.CollectionTracker, error) {
	entity, ok := c.Backend.ByID(collID)
	if !ok {
		return nil, fmt.Errorf("could not retrieve collection tracker from mempool")
	}
	collectionTracker := entity.(*tracker.CollectionTracker)
	return collectionTracker, nil
}

// All returns all collection trackers from the pool.
func (c *CollectionTrackers) All() []*tracker.CollectionTracker {
	c.Lock()
	defer c.Unlock()

	entities := c.Backend.All()
	collectionTrackers := make([]*tracker.CollectionTracker, 0, len(entities))
	for _, entity := range entities {
		collectionTrackers = append(collectionTrackers, entity.(*tracker.CollectionTracker))
	}
	return collectionTrackers
}
