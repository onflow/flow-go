package stdmap

import (
	"fmt"
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification/tracker"
)

// CollectionTrackers implements the CollectionTrackers memory pool.
type CollectionTrackers struct {
	sync.Mutex // used to provide atomic update functionality
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
func (c *CollectionTrackers) Add(ct *tracker.CollectionTracker) error {
	c.Lock()
	defer c.Unlock()

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
	c.Lock()
	defer c.Unlock()

	return c.Backend.Rem(collID)
}

// Inc atomically increases the counter of tracker by one and returns the updated tracker
func (c *CollectionTrackers) Inc(collID flow.Identifier) (*tracker.CollectionTracker, error) {
	c.Lock()
	defer c.Unlock()

	// retrieves tracker from mempool
	ct, err := c.byCollectionID(collID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve chunk data pack tracker from mempool: %w", err)
	}

	// removes it
	removed := c.Backend.Rem(collID)
	if !removed {
		return nil, fmt.Errorf("could not remove collection tracker from mempool")
	}

	// increases tracker retry counter
	ct.Counter += 1

	// inserts it back in the mempool
	err = c.Backend.Add(ct)
	if err != nil {
		return nil, fmt.Errorf("could not store tracker of collection request in mempool: %w", err)
	}

	return ct, nil
}

// byCollectionID is the non-concurrency safe version that
// returns the collection tracker for the given collection ID.
func (c *CollectionTrackers) byCollectionID(collID flow.Identifier) (*tracker.CollectionTracker, error) {
	entity, err := c.Backend.ByID(collID)
	if err != nil {
		return nil, err
	}
	collectionTracker, ok := entity.(*tracker.CollectionTracker)
	if !ok {
		return nil, fmt.Errorf("invalid entity in collection tracker pool (%T)", entity)
	}
	return collectionTracker, nil
}

// ByCollectionID returns the collection tracker for the given collection ID.
func (c *CollectionTrackers) ByCollectionID(collID flow.Identifier) (*tracker.CollectionTracker, error) {
	c.Lock()
	defer c.Unlock()

	return c.byCollectionID(collID)
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
