package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkStatuses is an in-memory storage for maintaining the chunk status data objects.
type ChunkStatuses struct {
	*Backend
}

func NewChunkStatuses(limit uint) *ChunkStatuses {
	chunks := &ChunkStatuses{
		Backend: NewBackend(WithLimit(limit)),
	}
	return chunks
}

func chunkStatus(entity flow.Entity) *verification.ChunkStatus {
	chunk, ok := entity.(*verification.ChunkStatus)
	if !ok {
		panic(fmt.Sprintf("could not convert the entity into chunk status from the mempool: %v", entity))
	}
	return chunk
}

func (cs *ChunkStatuses) All() []*verification.ChunkStatus {
	all := cs.Backend.All()
	allChunks := make([]*verification.ChunkStatus, 0, len(all))
	for _, entity := range all {
		chunk := chunkStatus(entity)
		allChunks = append(allChunks, chunk)
	}
	return allChunks
}

func (cs *ChunkStatuses) ByID(chunkID flow.Identifier) (*verification.ChunkStatus, bool) {
	entity, exists := cs.Backend.ByID(chunkID)
	if !exists {
		return nil, false
	}
	chunk := chunkStatus(entity)
	return chunk, true
}

func (cs *ChunkStatuses) Add(status *verification.ChunkStatus) bool {
	return cs.Backend.Add(status)
}

func (cs *ChunkStatuses) Rem(chunkID flow.Identifier) bool {
	return cs.Backend.Rem(chunkID)
}
