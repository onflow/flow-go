// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED
package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
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
func (c *ChunkDataPacks) Add(cdp *flow.ChunkDataPack) error {
	return c.Backend.Add(cdp)
}

// Rem will remove chunk data pack by ID
func (c *ChunkDataPacks) Rem(chunkID flow.Identifier) bool {
	return c.Backend.Rem(chunkID)
}

// ByChunkID returns the chunk data pack with the given chunkID from the mempool.
func (c *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	entity, err := c.Backend.ByID(chunkID)
	if err != nil {
		return nil, err
	}
	chunkDataPack, ok := entity.(*flow.ChunkDataPack)
	if !ok {
		return nil, fmt.Errorf("invalid entity in chunk data pack pool (%T)", entity)
	}
	return chunkDataPack, nil
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

// Hash will return a hash of the contents of the memory pool.
func (c *ChunkDataPacks) Hash() flow.Identifier {
	return c.Backend.Hash()
}
