package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataPacks implements the ChunkDataPack memory pool.
type ChunkDataPacks struct {
	*Backend
}

// NewChunkDataPacks creates a new memory pool for ChunkDataPacks.
func NewChunkDataPacks(limit uint) (*ChunkDataPacks, error) {
	a := &ChunkDataPacks{
		Backend: NewBackend(WithLimit(limit)),
	}
	return a, nil
}

// Has checks whether the ChunkDataPack with the given chunkID is currently in
// the memory pool.
func (c *ChunkDataPacks) Has(chunkID flow.Identifier) bool {
	return c.Backend.Has(chunkID)
}

// Add adds an chunkDataPack to the mempool.
func (c *ChunkDataPacks) Add(cdp *flow.ChunkDataPack) bool {
	added := c.Backend.Add(cdp)
	return added
}

// Remove will remove chunk data pack by ID
func (c *ChunkDataPacks) Remove(chunkID flow.Identifier) bool {
	removed := c.Backend.Remove(chunkID)
	return removed
}

// ByChunkID returns the chunk data pack with the given chunkID from the mempool.
func (c *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, bool) {
	entity, exists := c.Backend.ByID(chunkID)
	if !exists {
		return nil, false
	}
	chunkDataPack := entity.(*flow.ChunkDataPack)
	return chunkDataPack, true
}

// Size will return the current size of the memory pool.
func (c *ChunkDataPacks) Size() uint {
	return c.Backend.Size()
}

// All returns all chunk data packs from the pool.
func (c *ChunkDataPacks) All() []*flow.ChunkDataPack {
	entities := c.Backend.All()
	chunkDataPack := make([]*flow.ChunkDataPack, 0, len(entities))
	for _, entity := range entities {
		chunkDataPack = append(chunkDataPack, entity.(*flow.ChunkDataPack))
	}
	return chunkDataPack
}
