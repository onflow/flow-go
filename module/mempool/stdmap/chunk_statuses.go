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
	status, ok := entity.(inMemChunkStatus)
	if !ok {
		panic(fmt.Sprintf("could not convert the entity into chunk status from the mempool: %v", entity))
	}
	return &verification.ChunkStatus{
		ChunkIndex:      status.ChunkIndex,
		ExecutionResult: status.ExecutionResult,
		BlockHeight:     status.BlockHeight,
	}
}

// Get returns a chunk status by its chunk index and result ID.
// There is a one-to-one correspondence between the chunk statuses in memory, and
// their pair of chunk index and result id.
func (cs ChunkStatuses) Get(chunkIndex uint64, resultID flow.Identifier) (*verification.ChunkStatus, bool) {
	entity, exists := cs.Backend.ByID(chunks.ChunkLocatorID(resultID, chunkIndex))
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
	return cs.Backend.Add(inMemChunkStatus{
		ChunkIndex:      status.ChunkIndex,
		ExecutionResult: status.ExecutionResult,
		BlockHeight:     status.BlockHeight,
	})
}

// Rem provides deletion functionality from the memory pool based on the pair of
// chunk index and result id.
// If there is a chunk status associated with this pair, Rem removes it and returns true.
// Otherwise, it returns false.
func (cs *ChunkStatuses) Rem(chunkIndex uint64, resultID flow.Identifier) bool {
	return cs.Backend.Rem(chunks.ChunkLocatorID(resultID, chunkIndex))
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

// inMemChunkStatus is an internal type for storing ChunkStatus in the mempool.
//
// It is the same as ChunkStatus, but additionally it implements an Entity type which
// makes it storable in the mempool.
// Note that as an entity, the ID of a inMemChunkStatus is computed as the ID of the chunk locator
// it represents. However, the usage of ID method is only confined to maintaining it on the mempool.
// That is the motivation behind making it an internal type to make sure that no further decision out of
// this package is taken based on ID of inMemChunkStatus.
type inMemChunkStatus struct {
	ChunkIndex      uint64
	BlockHeight     uint64
	ExecutionResult *flow.ExecutionResult
}

func (s inMemChunkStatus) ID() flow.Identifier {
	return chunks.ChunkLocatorID(s.ExecutionResult.ID(), s.ChunkIndex)
}

func (s inMemChunkStatus) Checksum() flow.Identifier {
	return chunks.ChunkLocatorID(s.ExecutionResult.ID(), s.ChunkIndex)
}
