package stdmap

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module/mempool"
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

// ByID returns a chunk request by its chunk ID.
//
// There is a one-to-one correspondence between the chunk requests in memory, and
// their chunk ID.
func (cs *ChunkRequests) ByID(chunkID flow.Identifier) (*verification.ChunkDataPackRequest, bool) {
	entity, exists := cs.Backend.ByID(chunkID)
	if !exists {
		return nil, false
	}
	request := toChunkRequestStatus(entity)
	return request.ChunkDataPackRequest, true
}

// RequestInfo returns the number of times the chunk has been requested,
// last time the chunk has been requested, and the retry-after time duration of the
// underlying request status of this chunk.
//
// The last boolean parameter returns whether a chunk request for this chunk ID
// exists in memory-pool.
func (cs *ChunkRequests) RequestInfo(chunkID flow.Identifier) (uint64, time.Time, time.Duration, bool) {
	entity, exists := cs.Backend.ByID(chunkID)
	if !exists {
		return 0, time.Time{}, time.Duration(0), false
	}
	request := toChunkRequestStatus(entity)

	return request.Attempt, request.LastAttempt, request.RetryAfter, true
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

// UpdateRequestHistory updates the request history of the specified chunk ID. If the update was successful, i.e.,
// the updater returns true, the result of update is committed to the mempool, and the time stamp of the chunk request
// is updated to the current time. Otherwise, it aborts and returns false.
//
// The updates under this method are atomic, thread-safe, and done in isolation.
func (cs *ChunkRequests) UpdateRequestHistory(chunkID flow.Identifier, updater mempool.ChunkRequestHistoryUpdaterFunc) bool {
	err := cs.Backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, exists := backdata[chunkID]
		if !exists {
			return fmt.Errorf("not exist")
		}
		chunk := toChunkRequestStatus(entity)

		attempt, retryAfter, ok := updater(chunk.Attempt, chunk.RetryAfter)
		if !ok {
			return fmt.Errorf("updater failed")
		}

		chunk.LastAttempt = time.Now()
		chunk.RetryAfter = retryAfter
		chunk.Attempt = attempt

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
	LastAttempt time.Time     // timestamp of last request dispatched for this chunk id.
	RetryAfter  time.Duration // interval until request should be retried.
	Attempt     uint64
}

func (c chunkRequestStatus) ID() flow.Identifier {
	return c.ChunkID
}

func (c chunkRequestStatus) Checksum() flow.Identifier {
	return c.ChunkID
}
