package match

import (
	"fmt"
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
)

type ChunkStatus struct {
	Chunk             *flow.Chunk
	ExecutionResultID flow.Identifier
	ExecutorID        flow.Identifier
	LastAttempt       time.Time
	Attempt           int
}

func (s *ChunkStatus) ID() flow.Identifier {
	return s.Chunk.ID()
}

func (s *ChunkStatus) Checksum() flow.Identifier {
	return s.Chunk.ID()
}

func NewChunkStatus(chunk *flow.Chunk, resultID flow.Identifier, executorID flow.Identifier) *ChunkStatus {
	return &ChunkStatus{
		Chunk:             chunk,
		ExecutionResultID: resultID,
		ExecutorID:        executorID,
	}
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

func (cs *Chunks) All() []*ChunkStatus {
	all := cs.Backend.All()
	allChunks := make([]*ChunkStatus, 0, len(all))
	for _, entity := range all {
		chunk, _ := entity.(*ChunkStatus)
		allChunks = append(allChunks, chunk)
	}
	return allChunks
}

func (cs *Chunks) ByID(chunkID flow.Identifier) (*ChunkStatus, bool) {
	entity, exists := cs.Backend.ByID(chunkID)
	if !exists {
		return nil, false
	}
	chunk := entity.(*ChunkStatus)
	return chunk, true
}

func (cs *Chunks) Add(chunk *ChunkStatus) bool {
	return cs.Backend.Add(chunk)
}

func (cs *Chunks) Rem(chunkID flow.Identifier) bool {
	return cs.Backend.Rem(chunkID)
}

func (cs *Chunks) IncrementAttempt(chunkID flow.Identifier) bool {
	err := cs.Backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, exists := backdata[chunkID]
		if !exists {
			return fmt.Errorf("not exist")
		}
		chunk := entity.(*ChunkStatus)
		chunk.Attempt++
		chunk.LastAttempt = time.Now()
		return nil
	})

	return err == nil
}
