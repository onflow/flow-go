package mempool

import (
	"math"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkRequestHistoryUpdaterFunc is a function type that used by ChunkRequests mempool to perform atomic and isolated updates on the
// underlying chunk requests history.
type ChunkRequestHistoryUpdaterFunc func(uint64, time.Duration) (uint64, time.Duration, bool)

// ExponentialBackoffWithCutoffUpdater is a chunk request history updater factory that generates backoff of value
// interval^attempts - 1. For example, if interval = 2, then it returns a request backoff generator
// for the series of 2^attempt - 1.
//
// The generated backoff is substituted with the given cutoff value for backoff longer than the cutoff.
// This is to make sure that we do not backoff indefinitely.
func ExponentialBackoffWithCutoffUpdater(interval time.Duration, cutoff time.Duration) ChunkRequestHistoryUpdaterFunc {
	return func(attempts uint64, retryAfter time.Duration) (uint64, time.Duration, bool) {
		backoff := time.Duration(math.Pow(float64(interval), float64(attempts)) - 1)
		attempts++
		if backoff > cutoff {
			return attempts, cutoff, true
		}
		return attempts, backoff, true
	}
}

// IncrementalAttemptUpdater is a chunk request history updater factory that increments the attempt field of request status
// and makes it instantly available against any retry after qualifier.
func IncrementalAttemptUpdater() ChunkRequestHistoryUpdaterFunc {
	return func(attempts uint64, retryAfter time.Duration) (uint64, time.Duration, bool) {
		attempts++
		retryAfter = +time.Nanosecond // makes request instantly qualified against any retry after qualifier.
		return attempts, retryAfter, true
	}
}

// ChunkRequests is an in-memory storage for maintaining chunk data pack requests.
type ChunkRequests interface {
	// ByID returns a chunk request by its chunk ID.
	//
	// There is a one-to-one correspondence between the chunk requests in memory, and
	// their chunk ID.
	ByID(chunkID flow.Identifier) (*verification.ChunkDataPackRequest, bool)

	// RequestInfo returns the number of times the chunk has been requested,
	// last time the chunk has been requested, and the retry-after time duration of the
	// underlying request status of this chunk.
	//
	// The last boolean parameter returns whether a chunk request for this chunk ID
	// exists in memory-pool.
	RequestInfo(chunkID flow.Identifier) (uint64, time.Time, time.Duration, bool)

	// Add provides insertion functionality into the memory pool.
	// The insertion is only successful if there is no duplicate chunk request with the same
	// chunk ID in the memory. Otherwise, it aborts the insertion and returns false.
	Add(request *verification.ChunkDataPackRequest) bool

	// Rem provides deletion functionality from the memory pool.
	// If there is a chunk request with this ID, Rem removes it and returns true.
	// Otherwise it returns false.
	Rem(chunkID flow.Identifier) bool

	// IncrementAttempt increments the Attempt field of the corresponding status of the
	// chunk request in memory pool that has the specified chunk ID.
	// If such chunk ID does not exist in the memory pool, it returns false.
	//
	// The increments are done atomically, thread-safe, and in isolation.
	IncrementAttempt(chunkID flow.Identifier) bool

	// UpdateRequestHistory updates the request history of the specified chunk ID. If the update was successful, i.e.,
	// the updater returns true, the result of update is committed to the mempool, and the time stamp of the chunk request
	// is updated to the current time. Otherwise, it aborts and returns false.
	//
	// The updates under this method are atomic, thread-safe, and done in isolation.
	UpdateRequestHistory(chunkID flow.Identifier, updater ChunkRequestHistoryUpdaterFunc) bool

	// All returns all chunk requests stored in this memory pool.
	All() []*verification.ChunkDataPackRequest
}
