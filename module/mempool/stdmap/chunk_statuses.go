package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkStatuses is an implementation of in-memory storage for maintaining the chunk status data objects.
type ChunkStatuses struct {
	*Backend
}

func NewChunkStatuses(limit uint) *ChunkStatuses {
	return &ChunkStatuses{
		Backend: NewBackend(WithLimit(limit)),
	}
}

func chunkStatus(entity flow.Entity) *verification.ChunkStatus {
	chunk, ok := entity.(*verification.ChunkStatus)
	if !ok {
		panic(fmt.Sprintf("could not convert the entity into chunk status from the mempool: %v", entity))
	}
	return chunk
}

// ByID returns a chunk status by its chunk ID.
// There is a one-to-one correspondence between the chunk statuses in memory, and
// their chunk ID.
func (cs ChunkStatuses) Get(chunkIndex uint64, resultID flow.Identifier) (*verification.ChunkStatus, bool) {
	locatorID := chunks.Locator{
		ResultID: resultID,
		Index:    chunkIndex,
	}.ID()
	entity, exists := cs.Backend.ByID(locatorID)
	if !exists {
		return nil, false
	}
	status := chunkStatus(entity)
	return status, true
}

// Add provides insertion functionality into the memory pool.
// The insertion is only successful if there is no duplicate status with the same
// chunk ID in the memory. Otherwise, it aborts the insertion and returns false.
func (cs *ChunkStatuses) Add(status *verification.ChunkStatus) bool {
	return cs.Backend.Add(status)
}

// Rem provides deletion functionality from the memory pool based on the pair of
// chunk index and result id.
// If there is a chunk status associated with this pair, Rem removes it and returns true.
// Otherwise, it returns false.
func (cs *ChunkStatuses) Rem(chunkIndex uint64, resultID flow.Identifier) bool {
	locatorID := chunks.Locator{
		ResultID: resultID,
		Index:    chunkIndex,
	}.ID()
	return cs.Backend.Rem(locatorID)
}

// All returns all chunk statuses stored in this memory pool.
func (cs ChunkStatuses) All() []*verification.ChunkStatus {
	all := cs.Backend.All()
	statuses := make([]*verification.ChunkStatus, 0, len(all))
	for _, entity := range all {
		chunk := chunkStatus(entity)
		statuses = append(statuses, chunk)
	}
	return statuses
}

// Size returns total number of chunk statuses in the memory pool.
func (cs ChunkStatuses) Size() uint {
	return cs.Backend.Size()
}
