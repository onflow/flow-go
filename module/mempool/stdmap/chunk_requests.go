package stdmap

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkRequests is an implementation of in-memory storage for maintaining chunk requests data objects.
//
// In this implementation, the ChunkRequests
// wraps the ChunkDataPackRequests around an internal ChunkRequestStatus data object, and maintains the wrapped
// version in memory.
type ChunkRequests struct {
	*Backend
}

func NewChunkRequests(limit uint) *ChunkRequests {
	return &ChunkRequests{
		Backend: NewBackend(WithLimit(limit)),
	}
}

func toChunkRequestStatus(entity flow.Entity) *chunkRequestStatus {
	status, ok := entity.(*chunkRequestStatus)
	if !ok {
		panic(fmt.Sprintf("could not convert the entity into chunk status from the mempool: %v", entity))
	}
	return status
}

// ByID returns a chunk request by its chunk ID as well as the attempt field of its underlying
// chunk request status.
// There is a one-to-one correspondence between the chunk requests in memory, and
// their chunk ID.
func (cs *ChunkRequests) ByID(chunkID flow.Identifier) (*verification.ChunkDataPackRequest, int, bool) {
	entity, exists := cs.Backend.ByID(chunkID)
	if !exists {
		return nil, -1, false
	}
	request := toChunkRequestStatus(entity)
	return request.ChunkDataPackRequest, request.Attempt, true
}

// Add provides insertion functionality into the memory pool.
// The insertion is only successful if there is no duplicate chunk request with the same
// chunk ID in the memory. Otherwise, it aborts the insertion and returns false.
func (cs *ChunkRequests) Add(request *verification.ChunkDataPackRequest) bool {
	status := &chunkRequestStatus{
		ChunkDataPackRequest: request,
	}
	return cs.Backend.Add(status)
}

// Rem provides deletion functionality from the memory pool.
// If there is a chunk request with this ID, Rem removes it and returns true.
// Otherwise it returns false.
func (cs *ChunkRequests) Rem(chunkID flow.Identifier) bool {
	return cs.Backend.Rem(chunkID)
}

// IncrementAttempt increments the Attempt field of the corresponding status of the
// chunk request in memory pool that has the specified chunk ID.
// If such chunk ID does not exist in the memory pool, it returns false.
//
// The increments are done atomically, thread-safe, and in isolation.
func (cs *ChunkRequests) IncrementAttempt(chunkID flow.Identifier) bool {
	err := cs.Backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, exists := backdata[chunkID]
		if !exists {
			return fmt.Errorf("not exist")
		}
		chunk := toChunkRequestStatus(entity)
		chunk.Attempt++
		chunk.LastAttempt = time.Now()
		return nil
	})

	return err == nil
}

// All returns all chunk requests stored in this memory pool.
func (cs *ChunkRequests) All() []*verification.ChunkDataPackRequest {
	all := cs.Backend.All()
	requests := make([]*verification.ChunkDataPackRequest, 0, len(all))
	for _, entity := range all {
		status := toChunkRequestStatus(entity)
		requests = append(requests, status.ChunkDataPackRequest)
	}
	return requests
}

// chunkRequestStatus is an internal data type for ChunkRequests mempool. It acts as a wrapper for ChunkDataRequests, maintaining
// some auxiliary attributes that are internal to ChunkRequests.
type chunkRequestStatus struct {
	*verification.ChunkDataPackRequest
	LastAttempt time.Time
	Attempt     int
}

func (c chunkRequestStatus) ID() flow.Identifier {
	return c.ChunkID
}

func (c chunkRequestStatus) Checksum() flow.Identifier {
	return c.ChunkID
}
