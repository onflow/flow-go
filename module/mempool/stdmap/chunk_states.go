// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// ChunkStates implements the chunk state memory pool.
type ChunkStates struct {
	*Backend
}

// NewChunkStates creates a new memory pool for chunk states.
func NewChunkStates(limit uint) (*ChunkStates, error) {
	a := &ChunkStates{
		Backend: NewBackend(WithLimit(limit)),
	}

	return a, nil
}

// Add adds an chunkState to the mempool.
func (a *ChunkStates) Add(chunkState *flow.ChunkState) error {
	return a.Backend.Add(chunkState)
}

// ByID returns the chunk state with the given ID from the mempool.
func (a *ChunkStates) ByID(chunkID flow.Identifier) (*flow.ChunkState, error) {
	entity, err := a.Backend.ByID(chunkID)
	if err != nil {
		return nil, err
	}
	chunkState, ok := entity.(*flow.ChunkState)
	if !ok {
		panic(fmt.Sprintf("invalid entity in chunk state pool (%T)", entity))
	}
	return chunkState, nil
}

// All returns all chunk states from the pool.
func (a *ChunkStates) All() []*flow.ChunkState {
	entities := a.Backend.All()
	chunkStates := make([]*flow.ChunkState, 0, len(entities))
	for _, entity := range entities {
		chunkStates = append(chunkStates, entity.(*flow.ChunkState))
	}
	return chunkStates
}
