package stdmap

import (
	"fmt"
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification/tracker"
)

// ChunkDataPackTrackers implements the ChunkDataPackTrackers memory pool.
type ChunkDataPackTrackers struct {
	sync.Mutex // used for preserving mutual exclusion during update
	*Backend
}

// NewChunkDataPackTrackers creates a new memory pool for ChunkDataPackTrackers.
func NewChunkDataPackTrackers(limit uint) (*ChunkDataPackTrackers, error) {
	a := &ChunkDataPackTrackers{
		Backend: NewBackend(WithLimit(limit)),
	}
	return a, nil
}

// Add adds a ChunkDataPackTracker to the mempool.
func (c *ChunkDataPackTrackers) Add(cdpt *tracker.ChunkDataPackTracker) error {
	c.Lock()
	defer c.Unlock()
	return c.Backend.Add(cdpt)
}

// Has checks whether the ChunkDataPackTracker with the given chunkID is currently in
// the memory pool.
func (c *ChunkDataPackTrackers) Has(chunkID flow.Identifier) bool {
	c.Lock()
	defer c.Unlock()
	return c.Backend.Has(chunkID)
}

// Rem removes tracker with the given chunk ID.
func (c *ChunkDataPackTrackers) Rem(chunkID flow.Identifier) bool {
	c.Lock()
	defer c.Unlock()
	return c.Backend.Rem(chunkID)
}

// Inc atomically increases the counter of tracker by one and returns the counter
func (c *ChunkDataPackTrackers) Inc(chunkID flow.Identifier) (*tracker.ChunkDataPackTracker, error) {
	c.Lock()
	defer c.Unlock()

	// retrieves the chunk data pack
	t, err := c.byChunkID(chunkID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve chunk data pack tracker from mempool: %w", err)
	}

	// removes it from memory
	removed := c.Backend.Rem(chunkID)
	if !removed {
		return nil, fmt.Errorf("could not remove data pack tracker from mempool")
	}

	// increases tracker retry counter
	t.Counter += 1

	// stores tracker back in the mempool
	err = c.Backend.Add(t)
	if err != nil {
		return nil, fmt.Errorf("could not store tracker of chunk data pack request in mempool: %w", err)
	}

	return t, nil
}

// byChunkID is the non-concurrency safe version that is used internally
// it returns the chunk data pack tracker for the given chunk ID.
func (c *ChunkDataPackTrackers) byChunkID(chunkID flow.Identifier) (*tracker.ChunkDataPackTracker, error) {
	entity, err := c.Backend.ByID(chunkID)
	if err != nil {
		return nil, err
	}
	chunkDataPackTracker, ok := entity.(*tracker.ChunkDataPackTracker)
	if !ok {
		return nil, fmt.Errorf("invalid entity in chunk data pack tracker pool (%T)", entity)
	}
	return chunkDataPackTracker, nil
}

// ByChunkID returns the chunk data pack tracker for the given chunk ID.
func (c *ChunkDataPackTrackers) ByChunkID(chunkID flow.Identifier) (*tracker.ChunkDataPackTracker, error) {
	c.Lock()
	defer c.Unlock()

	return c.byChunkID(chunkID)
}

// All returns all chunk data pack trackers from the pool.
func (c *ChunkDataPackTrackers) All() []*tracker.ChunkDataPackTracker {
	c.Lock()
	defer c.Unlock()

	entities := c.Backend.All()
	chunkDataPackTrackers := make([]*tracker.ChunkDataPackTracker, 0, len(entities))
	for _, entity := range entities {
		chunkDataPackTrackers = append(chunkDataPackTrackers, entity.(*tracker.ChunkDataPackTracker))
	}
	return chunkDataPackTrackers
}
