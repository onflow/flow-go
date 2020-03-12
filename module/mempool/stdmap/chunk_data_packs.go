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

// Add adds an chunkDataPack to the mempool.
func (a *ChunkDataPacks) Add(chunkDataPack *flow.ChunkDataPack) error {
	return a.Backend.Add(chunkDataPack)
}

// ByID returns the chunk data pack with the given ID from the mempool.
func (a *ChunkDataPacks) ByID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	entity, err := a.Backend.ByID(chunkID)
	if err != nil {
		return nil, err
	}
	chunkDataPack, ok := entity.(*flow.ChunkDataPack)
	if !ok {
		panic(fmt.Sprintf("invalid entity in chunk data pack pool (%T)", entity))
	}
	return chunkDataPack, nil
}

// All returns all chunk data packs from the pool.
func (a *ChunkDataPacks) All() []*flow.ChunkDataPack {
	entities := a.Backend.All()
	chunkDataPack := make([]*flow.ChunkDataPack, 0, len(entities))
	for _, entity := range entities {
		chunkDataPack = append(chunkDataPack, entity.(*flow.ChunkDataPack))
	}
	return chunkDataPack
}
