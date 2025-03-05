package stdmap

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkStatuses is an implementation of in-memory storage for maintaining the chunk status data objects.
type ChunkStatuses struct {
	*Backend[flow.Identifier, *verification.ChunkStatus]
}

func NewChunkStatuses(limit uint) *ChunkStatuses {
	return &ChunkStatuses{
		Backend: NewBackend(WithLimit[flow.Identifier, *verification.ChunkStatus](limit)),
	}
}

// Get returns a chunk status by its chunk index and result ID.
// There is a one-to-one correspondence between the chunk statuses in memory, and
// their pair of chunk index and result id.
func (cs ChunkStatuses) Get(chunkIndex uint64, resultID flow.Identifier) (*verification.ChunkStatus, bool) {
	status, exists := cs.Backend.Get(chunks.ChunkLocatorID(resultID, chunkIndex))
	if !exists {
		return nil, false
	}

	return status, true
}

// Add provides insertion functionality into the memory pool.
// The insertion is only successful if there is no duplicate status with the same
// chunk ID in the memory. Otherwise, it aborts the insertion and returns false.
func (cs *ChunkStatuses) Add(status *verification.ChunkStatus) bool {
	return cs.Backend.Add(chunks.ChunkLocatorID(status.ExecutionResult.ID(), status.ChunkIndex), status)
}

// Remove provides deletion functionality from the memory pool based on the pair of
// chunk index and result id.
// If there is a chunk status associated with this pair, Remove removes it and returns true.
// Otherwise, it returns false.
func (cs *ChunkStatuses) Remove(chunkIndex uint64, resultID flow.Identifier) bool {
	return cs.Backend.Remove(chunks.ChunkLocatorID(resultID, chunkIndex))
}

// All returns all chunk statuses stored in this memory pool.
func (cs ChunkStatuses) All() []*verification.ChunkStatus {
	all := cs.Backend.All()
	statuses := make([]*verification.ChunkStatus, 0, len(all))
	for _, status := range all {
		statuses = append(statuses, status)
	}
	return statuses
}

// Size returns total number of chunk statuses in the memory pool.
func (cs ChunkStatuses) Size() uint {
	return cs.Backend.Size()
}
