package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataPacks implements the ChunkDataPack memory pool.
type ChunkDataPacks struct {
	*Backend[flow.Identifier, *flow.ChunkDataPack]
}

// NewChunkDataPacks creates a new memory pool for ChunkDataPacks.
func NewChunkDataPacks(limit uint) (*ChunkDataPacks, error) {
	a := &ChunkDataPacks{
		Backend: NewBackend(WithLimit[flow.Identifier, *flow.ChunkDataPack](limit)),
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
	added := c.Backend.Add(cdp.ID(), cdp)
	return added
}

// Remove will remove chunk data pack by ID
func (c *ChunkDataPacks) Remove(chunkID flow.Identifier) bool {
	removed := c.Backend.Remove(chunkID)
	return removed
}

// ByChunkID returns the chunk data pack with the given chunkID from the mempool.
func (c *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, bool) {
	chunkDataPack, exists := c.Backend.ByID(chunkID)
	if !exists {
		return nil, false
	}
	return chunkDataPack, true
}

// Size will return the current size of the memory pool.
func (c *ChunkDataPacks) Size() uint {
	return c.Backend.Size()
}

// All returns all chunk data packs from the pool.
func (c *ChunkDataPacks) All() []*flow.ChunkDataPack {
	entities := c.Backend.All()
	chunkDataPacks := make([]*flow.ChunkDataPack, 0, len(entities))
	for _, chunkDataPack := range entities {
		chunkDataPacks = append(chunkDataPacks, chunkDataPack)
	}
	return chunkDataPacks
}
