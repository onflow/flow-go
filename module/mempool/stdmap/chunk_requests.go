package stdmap

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/chunks"
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

// RequestHistory returns the number of times the chunk has been requested,
// last time the chunk has been requested, and the retryAfter duration of the
// underlying request status of this chunk.
//
// The last boolean parameter returns whether a chunk request for this chunk ID
// exists in memory-pool.
func (cs *ChunkRequests) RequestHistory(chunkID flow.Identifier) (uint64, time.Time, time.Duration, bool) {
	var lastAttempt time.Time
	var retryAfter time.Duration
	var attempts uint64

	err := cs.Backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, ok := backdata[chunkID]
		if !ok {
			return fmt.Errorf("request does not exist for chunk %x", chunkID)
		}

		request := toChunkRequestStatus(entity)
		lastAttempt = request.LastAttempt
		retryAfter = request.RetryAfter
		attempts = request.Attempt
		return nil
	})

	return attempts, lastAttempt, retryAfter, err == nil
}

// Add provides insertion functionality into the memory pool.
// The insertion is only successful if there is no duplicate chunk request for the same
// tuple of (chunkID, resultID, chunkIndex).
func (cs *ChunkRequests) Add(request *verification.ChunkDataPackRequest) bool {
	err := cs.Backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, exists := backdata[request.ChunkID]

		if !exists {
			// no chunk request status exists for this chunk ID, hence initiating one.
			status := &chunkRequestStatus{
				Locators:    chunks.LocatorList{&request.Locator},
				RequestInfo: request.ChunkDataPackRequestInfo,
			}
			if ok := cs.Backend.Add(status); !ok {
				return fmt.Errorf("chunk request already exists")
			}
			return nil
		}

		statuses := toChunkRequestStatus(entity)
		if statuses.Locators.Contains(&request.Locator) {
			return fmt.Errorf("chunk request exists with same locator")
		}

		statuses.Locators = append(statuses.Locators, &request.Locator)
		statuses.RequestInfo.Agrees.Union(request.Agrees)
		statuses.RequestInfo.Disagrees.Union(request.Disagrees)
		return nil
	})

	return err != nil
}

// Rem provides deletion functionality from the memory pool.
// If there is a chunk request with this ID, Rem removes it and returns true.
// Otherwise it returns false.
func (cs *ChunkRequests) Rem(chunkID flow.Identifier) bool {
	return cs.Backend.Rem(chunkID)
}

// PopAll atomically returns all locators associated with this chunk ID while clearing out the
// chunk request status for this chunk id.
// Boolean return value indicates whether there are requests in the memory pool associated
// with chunk ID.
func (cs *ChunkRequests) PopAll(chunkID flow.Identifier) (chunks.LocatorList, bool) {
	var locators chunks.LocatorList

	err := cs.Backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, exists := backdata[chunkID]
		if !exists {
			return fmt.Errorf("not exist")
		}
		locators = toChunkRequestStatus(entity).Locators

		delete(backdata, chunkID)

		return nil
	})

	if err != nil {
		return nil, false
	}

	return locators, true
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
func (cs *ChunkRequests) All() verification.ChunkDataPackRequestInfoList {
	all := cs.Backend.All()
	requestInfoList := verification.ChunkDataPackRequestInfoList{}
	for _, entity := range all {
		requestInfo := toChunkRequestStatus(entity).RequestInfo
		requestInfoList = append(requestInfoList, &requestInfo)
	}
	return requestInfoList
}

// UpdateRequestHistory updates the request history of the specified chunk ID. If the update was successful, i.e.,
// the updater returns true, the result of update is committed to the mempool, and the time stamp of the chunk request
// is updated to the current time. Otherwise, it aborts and returns false.
//
// It returns the updated request history values.
//
// The updates under this method are atomic, thread-safe, and done in isolation.
func (cs *ChunkRequests) UpdateRequestHistory(chunkID flow.Identifier, updater mempool.ChunkRequestHistoryUpdaterFunc) (uint64, time.Time, time.Duration, bool) {
	var lastAttempt time.Time
	var retryAfter time.Duration
	var attempts uint64

	err := cs.Backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, exists := backdata[chunkID]
		if !exists {
			return fmt.Errorf("not exist")
		}
		status := toChunkRequestStatus(entity)

		var ok bool
		attempts, retryAfter, ok = updater(status.Attempt, status.RetryAfter)
		if !ok {
			return fmt.Errorf("updater failed")
		}
		lastAttempt = time.Now()

		// updates underlying request
		status.LastAttempt = lastAttempt
		status.RetryAfter = retryAfter
		status.Attempt = attempts

		return nil
	})

	return attempts, lastAttempt, retryAfter, err == nil
}

// Size returns total number of chunk requests in the memory pool.
func (cs ChunkRequests) Size() uint {
	return cs.Backend.Size()
}

// chunkRequestStatus is an internal data type for ChunkRequests mempool. It acts as a wrapper for ChunkDataRequests, maintaining
// some auxiliary attributes that are internal to ChunkRequests.
type chunkRequestStatus struct {
	Locators    chunks.LocatorList
	RequestInfo verification.ChunkDataPackRequestInfo
	LastAttempt time.Time     // timestamp of last request dispatched for this chunk id.
	RetryAfter  time.Duration // interval until request should be retried.
	Attempt     uint64        // number of times this chunk request has been dispatched in the network.
}

func (c chunkRequestStatus) ID() flow.Identifier {
	return c.RequestInfo.ChunkID
}

func (c chunkRequestStatus) Checksum() flow.Identifier {
	return c.RequestInfo.ChunkID
}
