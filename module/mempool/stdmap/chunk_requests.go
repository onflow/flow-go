package stdmap

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkRequests is an implementation of in-memory storage for maintaining chunk requests data objects.
type ChunkRequests struct {
	*Backend
}

func NewChunkRequests(limit uint) *ChunkRequests {
	return &ChunkRequests{
		Backend: NewBackend(WithLimit(limit)),
	}
}

func chunkRequestStatus(entity flow.Entity) *verification.ChunkRequestStatus {
	chunk, ok := entity.(*verification.ChunkRequestStatus)
	if !ok {
		panic(fmt.Sprintf("could not convert the entity into chunk status from the mempool: %v", entity))
	}
	return chunk
}

// ByID returns a chunk request status by its chunk ID.
// There is a one-to-one correspondence between the chunk requests in memory, and
// their chunk ID.
func (cs *ChunkRequests) ByID(chunkID flow.Identifier) (*verification.ChunkRequestStatus, bool) {
	entity, exists := cs.Backend.ByID(chunkID)
	if !exists {
		return nil, false
	}
	request := chunkRequestStatus(entity)
	return request, true
}

// Add provides insertion functionality into the memory pool.
// The insertion is only successful if there is no duplicate chunk request with the same
// chunk ID in the memory. Otherwise, it aborts the insertion and returns false.
func (cs *ChunkRequests) Add(request *verification.ChunkRequestStatus) bool {
	return cs.Backend.Add(request)
}

// Rem provides deletion functionality from the memory pool.
// If there is a chunk request with this ID, Rem removes it and returns true.
// Otherwise it returns false.
func (cs *ChunkRequests) Rem(chunkID flow.Identifier) bool {
	return cs.Backend.Rem(chunkID)
}

// IncrementAttempt increments the Attempt field of the chunk request in memory pool that
// has the specified chunk ID. If such chunk ID does not exist in the memory pool, it returns
// false.
// The increments are done atomically, thread-safe, and in isolation.
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

// IncrementAttemptAndRetryAfter increments the Attempt field of the chunk request in memory pool that
// has the specified chunk ID, and updates the retryAfter field of the chunk request to the specified
// values.
//
// The LastAttempt field of the chunk request is timestamped with the invocation time of this method.
//
// If such chunk ID does not exist in the memory pool, it returns false.
// The updates under this method are atomic, thread-safe, and done in isolation.
func (cs *ChunkRequests) IncrementAttemptAndRetryAfter(chunkID flow.Identifier, retryAfter time.Duration) bool {
	err := cs.Backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, exists := backdata[chunkID]
		if !exists {
			return fmt.Errorf("not exist")
		}
		chunk := chunkRequestStatus(entity)
		chunk.Attempt++
		chunk.LastAttempt = time.Now()
		chunk.RetryAfter = retryAfter
		return nil
	})

	return err == nil
}

// All returns all chunk requests stored in this memory pool.
func (cs *ChunkRequests) All() []*verification.ChunkRequestStatus {
	all := cs.Backend.All()
	requests := make([]*verification.ChunkRequestStatus, 0, len(all))
	for _, entity := range all {
		chunk := chunkRequestStatus(entity)
		requests = append(requests, chunk)
	}
	return requests
}
