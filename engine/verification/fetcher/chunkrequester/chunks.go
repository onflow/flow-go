package chunkrequester

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

// ChunkRequestStatus is a data struct represents the current status of fetching
// chunk data pack for the chunk.
type ChunkRequestStatus struct {
	*fetcher.ChunkDataPackRequest
	Targets     flow.IdentityList
	LastAttempt time.Time
	Attempt     int
}

func (c ChunkRequestStatus) ID() flow.Identifier {
	return c.ChunkID
}

func (c ChunkRequestStatus) Checksum() flow.Identifier {
	return c.ChunkID
}

// SampleTargets returns identifier of execution nodes that can be asked for the chunk data pack, based on
// the agree and disagree execution nodes of the chunk data pack request.
func (c ChunkRequestStatus) SampleTargets(count int) flow.IdentifierList {
	// if there are enough receipts produced the same result (agrees), we sample from them.
	if len(c.Agrees) >= count {
		return c.Targets.Filter(filter.HasNodeID(c.Agrees...)).Sample(uint(count)).NodeIDs()
	}

	// since there is at least one agree, then usually, we just need `count - 1` extra nodes as backup.
	// We pick these extra nodes randomly from the rest nodes who we haven't received its receipt.
	// In the case where all other execution nodes has produced different results, then we will only
	// fetch from the one produced the same result (the only agree)
	need := uint(count - len(c.Agrees))

	nonResponders := c.Targets.Filter(filter.Not(filter.HasNodeID(c.Disagrees...))).Sample(need).NodeIDs()
	return append(c.Agrees, nonResponders...)
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
