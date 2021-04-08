package stdmap

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

type ChunkRequests struct {
	*Backend
}

func NewChunkRequests(limit uint) *ChunkRequests {
	chunks := &ChunkRequests{
		Backend: NewBackend(WithLimit(limit)),
	}
	return chunks
}

func chunkRequestStatus(entity flow.Entity) *verification.ChunkRequestStatus {
	chunk, ok := entity.(*verification.ChunkRequestStatus)
	if !ok {
		panic(fmt.Sprintf("could not convert the entity into chunk status from the mempool: %v", entity))
	}
	return chunk
}

func (cs *ChunkRequests) All() []*verification.ChunkRequestStatus {
	all := cs.Backend.All()
	allChunks := make([]*verification.ChunkRequestStatus, 0, len(all))
	for _, entity := range all {
		chunk := chunkRequestStatus(entity)
		allChunks = append(allChunks, chunk)
	}
	return allChunks
}

func (cs *ChunkRequests) ByID(chunkID flow.Identifier) (*verification.ChunkRequestStatus, bool) {
	entity, exists := cs.Backend.ByID(chunkID)
	if !exists {
		return nil, false
	}
	chunk := chunkRequestStatus(entity)
	return chunk, true
}

func (cs *ChunkRequests) Add(chunk *verification.ChunkRequestStatus) bool {
	return cs.Backend.Add(chunk)
}

func (cs *ChunkRequests) Rem(chunkID flow.Identifier) bool {
	return cs.Backend.Rem(chunkID)
}

func (cs *ChunkRequests) IncrementAttempt(chunkID flow.Identifier) bool {
	err := cs.Backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, exists := backdata[chunkID]
		if !exists {
			return fmt.Errorf("not exist")
		}
		chunk := chunkRequestStatus(entity)
		chunk.Attempt++
		chunk.LastAttempt = time.Now()
		return nil
	})

	return err == nil
}
