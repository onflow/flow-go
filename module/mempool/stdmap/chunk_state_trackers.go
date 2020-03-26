package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification/tracker"
)

// ChunkStateTrackers implements the ChunkStateTrackers memory pool.
type ChunkStateTrackers struct {
	*Backend
}

// NewChunkStateTrackers creates a new memory pool for ChunkStateTrackers.
func NewChunkStateTrackers(limit uint) (*ChunkStateTrackers, error) {
	a := &ChunkStateTrackers{
		Backend: NewBackend(WithLimit(limit)),
	}
	return a, nil
}

// Add adds a ChunkStateTracker to the mempool.
func (c *ChunkStateTrackers) Add(cst *tracker.ChunkStateTracker) error {
	return c.Backend.Add(cst)
}

// Has checks whether the ChunkStateTracker with the given chunkID is currently in
// the memory pool.
func (c *ChunkStateTrackers) Has(chunkID flow.Identifier) bool {
	return c.Backend.Has(chunkID)
}

// Rem removes tracker with the given chunk ID.
func (c *ChunkStateTrackers) Rem(chunkID flow.Identifier) bool {
	return c.Backend.Rem(chunkID)
}

// ByChunkID returns the chunk state tracker for the given chunk ID.
func (c *ChunkStateTrackers) ByChunkID(chunkID flow.Identifier) (*tracker.ChunkStateTracker, error) {
	entity, err := c.Backend.ByID(chunkID)
	if err != nil {
		return nil, err
	}
	chunkStateTracker, ok := entity.(*tracker.ChunkStateTracker)
	if !ok {
		return nil, fmt.Errorf("invalid entity in chunk state tracker pool (%T)", entity)
	}
	return chunkStateTracker, nil
}

// All returns all chunk state trackers from the pool.
func (c *ChunkStateTrackers) All() []*tracker.ChunkStateTracker {
	entities := c.Backend.All()
	chunkStateTracker := make([]*tracker.ChunkStateTracker, 0, len(entities))
	for _, entity := range entities {
		chunkStateTracker = append(chunkStateTracker, entity.(*tracker.ChunkStateTracker))
	}
	return chunkStateTracker
}
