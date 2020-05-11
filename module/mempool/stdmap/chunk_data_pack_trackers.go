package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification/tracker"
)

// ChunkDataPackTrackers implements the ChunkDataPackTrackers memory pool.
type ChunkDataPackTrackers struct {
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
func (c *ChunkDataPackTrackers) Add(cdpt *tracker.ChunkDataPackTracker) bool {
	return c.Backend.Add(cdpt)
}

// Has checks whether the ChunkDataPackTracker with the given chunkID is currently in
// the memory pool.
func (c *ChunkDataPackTrackers) Has(chunkID flow.Identifier) bool {
	return c.Backend.Has(chunkID)
}

// Rem removes tracker with the given chunk ID.
func (c *ChunkDataPackTrackers) Rem(chunkID flow.Identifier) bool {
	return c.Backend.Rem(chunkID)
}

// Inc atomically increases the counter of tracker by one and returns the updated tracker
func (c *ChunkDataPackTrackers) Inc(chunkID flow.Identifier) (*tracker.ChunkDataPackTracker, error) {
	updated, ok := c.Backend.Adjust(chunkID, func(entity flow.Entity) flow.Entity {
		cdpt := entity.(*tracker.ChunkDataPackTracker)
		return &tracker.ChunkDataPackTracker{
			ChunkID: cdpt.ChunkID,
			BlockID: cdpt.BlockID,
			Counter: cdpt.Counter + 1,
		}
	})

	if !ok {
		return nil, fmt.Errorf("could not update tracker in backend")
	}

	return updated.(*tracker.ChunkDataPackTracker), nil
}

// ByChunkID returns the chunk data pack tracker for the given chunk ID.
func (c *ChunkDataPackTrackers) ByChunkID(chunkID flow.Identifier) (*tracker.ChunkDataPackTracker, error) {
	entity, ok := c.Backend.ByID(chunkID)
	if !ok {
		return nil, fmt.Errorf("could not retrieve tracker from backend")
	}
	chunkDataPackTracker := entity.(*tracker.ChunkDataPackTracker)
	return chunkDataPackTracker, nil
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
