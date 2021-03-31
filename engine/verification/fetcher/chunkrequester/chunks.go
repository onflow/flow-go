package chunkrequester

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

// ChunkRequestStatus is a data struct represents the current status of fetching
// chunk data pack for the chunk.
type ChunkRequestStatus struct {
	*fetcher.ChunkDataPackRequest
	LastAttempt time.Time
	Attempt     int
}

func (s ChunkRequestStatus) ID() flow.Identifier {
	return s.ChunkID
}

func (s ChunkRequestStatus) Checksum() flow.Identifier {
	return s.ChunkID
}

type ChunkRequests struct {
	*stdmap.Backend
}

func NewChunkRequests(limit uint) *ChunkRequests {
	chunks := &ChunkRequests{
		Backend: stdmap.NewBackend(stdmap.WithLimit(limit)),
	}
	return chunks
}

func fromEntity(entity flow.Entity) *ChunkRequestStatus {
	chunk, ok := entity.(*ChunkRequestStatus)
	if !ok {
		panic(fmt.Sprintf("could not convert the entity into chunk status from the mempool: %v", entity))
	}
	return chunk
}

func (cs *ChunkRequests) All() []*ChunkRequestStatus {
	all := cs.Backend.All()
	allChunks := make([]*ChunkRequestStatus, 0, len(all))
	for _, entity := range all {
		chunk := fromEntity(entity)
		allChunks = append(allChunks, chunk)
	}
	return allChunks
}

func (cs *ChunkRequests) ByID(chunkID flow.Identifier) (*ChunkRequestStatus, bool) {
	entity, exists := cs.Backend.ByID(chunkID)
	if !exists {
		return nil, false
	}
	chunk := fromEntity(entity)
	return chunk, true
}

func (cs *ChunkRequests) Add(chunk *ChunkRequestStatus) bool {
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
		chunk := fromEntity(entity)
		chunk.Attempt++
		chunk.LastAttempt = time.Now()
		return nil
	})

	return err == nil
}
