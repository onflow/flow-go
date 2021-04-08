package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

type Chunks struct {
	*Backend
}

func NewChunkStatuses(limit uint) *Chunks {
	chunks := &Chunks{
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

func (cs *Chunks) All() []*verification.ChunkStatus {
	all := cs.Backend.All()
	allChunks := make([]*verification.ChunkStatus, 0, len(all))
	for _, entity := range all {
		chunk := chunkStatus(entity)
		allChunks = append(allChunks, chunk)
	}
	return allChunks
}

func (cs *Chunks) ByID(chunkID flow.Identifier) (*verification.ChunkStatus, bool) {
	entity, exists := cs.Backend.ByID(chunkID)
	if !exists {
		return nil, false
	}
	chunk := chunkStatus(entity)
	return chunk, true
}

func (cs *Chunks) Add(chunk *verification.ChunkStatus) bool {
	return cs.Backend.Add(chunk)
}

func (cs *Chunks) Rem(chunkID flow.Identifier) bool {
	return cs.Backend.Rem(chunkID)
}
