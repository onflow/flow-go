package fetcher

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

// ChunkStatus is a data struct represents the current status of fetching chunk data pack for the chunk.
type ChunkStatus struct {
	Chunk             *flow.Chunk
	ExecutionResultID flow.Identifier
}

func (s ChunkStatus) ID() flow.Identifier {
	return s.Chunk.ID()
}

func (s ChunkStatus) Checksum() flow.Identifier {
	return s.Chunk.ID()
}

type Chunks struct {
	*stdmap.Backend
}

func NewChunks(limit uint) *Chunks {
	chunks := &Chunks{
		Backend: stdmap.NewBackend(stdmap.WithLimit(limit)),
	}
	return chunks
}

func fromEntity(entity flow.Entity) *ChunkStatus {
	chunk, ok := entity.(*ChunkStatus)
	if !ok {
		panic(fmt.Sprintf("could not convert the entity into chunk status from the mempool: %v", entity))
	}
	return chunk
}

func (cs *Chunks) All() []*ChunkStatus {
	all := cs.Backend.All()
	allChunks := make([]*ChunkStatus, 0, len(all))
	for _, entity := range all {
		chunk := fromEntity(entity)
		allChunks = append(allChunks, chunk)
	}
	return allChunks
}

func (cs *Chunks) ByID(chunkID flow.Identifier) (*ChunkStatus, bool) {
	entity, exists := cs.Backend.ByID(chunkID)
	if !exists {
		return nil, false
	}
	chunk := fromEntity(entity)
	return chunk, true
}

func (cs *Chunks) Add(chunk *ChunkStatus) bool {
	return cs.Backend.Add(chunk)
}

func (cs *Chunks) Rem(chunkID flow.Identifier) bool {
	return cs.Backend.Rem(chunkID)
}
