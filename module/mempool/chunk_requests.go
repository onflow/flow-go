package mempool

import (
	"time"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkRequestHistoryUpdaterFunc is a function type that used by ChunkRequests mempool to perform atomic and isolated updates on the
// underlying chunk requests history.
type ChunkRequestHistoryUpdaterFunc func(uint64, time.Duration) (uint64, time.Duration, bool)

// ExponentialUpdater is a chunk request history updater factory that updates the retryAfter value of a request to
// multiplier * retryAfter. For example, if multiplier = 2,
// then invoking it n times results in a retryAfter value of 2^n * retryAfter, which follows an exponential series.
//
// It also keeps updated retryAfter value between the minInterval and maxInterval inclusive. It means that if updated retryAfter value
// is below minInterval, it is bumped up to the minInterval. Also, if updated retryAfter value is above maxInterval, it is skimmed off back
// to the maxInterval.
//
// Note: if initial retryAfter is below minInterval, the first call to this function returns minInterval, and hence after the nth invocations,
// the retryAfter value is set to 2^(n-1) * minInterval.
func ExponentialUpdater(multiplier float64, maxInterval time.Duration, minInterval time.Duration) ChunkRequestHistoryUpdaterFunc {
	return func(attempts uint64, retryAfter time.Duration) (uint64, time.Duration, bool) {
		if float64(retryAfter) >= float64(maxInterval)/multiplier {
			retryAfter = maxInterval
		} else {
			retryAfter = time.Duration(float64(retryAfter) * multiplier)

			if retryAfter < minInterval {
				retryAfter = minInterval
			}
		}

		return attempts + 1, retryAfter, true
	}
}

// IncrementalAttemptUpdater is a chunk request history updater factory that increments the attempt field of request status
// and makes it instantly available against any retryAfter qualifier.
func IncrementalAttemptUpdater() ChunkRequestHistoryUpdaterFunc {
	return func(attempts uint64, _ time.Duration) (uint64, time.Duration, bool) {
		attempts++
		// makes request instantly qualified against any retry after qualifier.
		return attempts, time.Nanosecond, true
	}
}

// ChunkRequests is an in-memory storage for maintaining chunk data pack requests.
type ChunkRequests interface {
	// RequestHistory returns the number of times the chunk has been requested,
	// last time the chunk has been requested, and the retryAfter duration of the
	// underlying request status of this chunk.
	//
	// The last boolean parameter returns whether a chunk request for this chunk ID
	// exists in memory-pool.
	RequestHistory(chunkID flow.Identifier) (uint64, time.Time, time.Duration, bool)

	// Add provides insertion functionality into the memory pool.
	// The insertion is only successful if there is no duplicate chunk request with the same
	// chunk ID in the memory. Otherwise, it aborts the insertion and returns false.
	Add(request *verification.ChunkDataPackRequest) bool

	// Rem provides deletion functionality from the memory pool.
	// If there is a chunk request with this ID, Rem removes it and returns true.
	// Otherwise, it returns false.
	Rem(chunkID flow.Identifier) bool

	// PopAll atomically returns all locators associated with this chunk ID while clearing out the
	// chunk request status for this chunk id.
	// Boolean return value indicates whether there are requests in the memory pool associated
	// with chunk ID.
	PopAll(chunkID flow.Identifier) (chunks.LocatorList, bool)

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
	// It returns the updated request history values.
	//
	// The updates under this method are atomic, thread-safe, and done in isolation.
	UpdateRequestHistory(chunkID flow.Identifier, updater ChunkRequestHistoryUpdaterFunc) (uint64, time.Time, time.Duration, bool)

	// All returns all chunk requests stored in this memory pool.
	All() verification.ChunkDataPackRequestInfoList

	// Size returns total number of chunk requests in the memory pool.
	Size() uint
}
