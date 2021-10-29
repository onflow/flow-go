package stdmap_test

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/verification/requester"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestChunkRequests_UpdateRequestHistory evaluates behavior of ChuckRequests against updating request histories with
// different updaters.
func TestChunkRequests_UpdateRequestHistory(t *testing.T) {
	qualifier := requester.RetryAfterQualifier
	t.Run("10 chunks- 10 times incremental updater ", func(t *testing.T) {
		incUpdater := mempool.IncrementalAttemptUpdater()
		chunks := 10
		expectedAttempts := 10

		withUpdaterScenario(t, chunks, expectedAttempts, incUpdater, func(t *testing.T, attempts uint64, lastTried time.Time, retryAfter time.Duration) {
			require.Equal(t, expectedAttempts, int(attempts))           // each chunk request should be attempted 10 times.
			require.True(t, qualifier(attempts, lastTried, retryAfter)) // request should be immediately qualified for retrial.
		})
	})

	t.Run("10 chunks- 10 times exponential updater", func(t *testing.T) {
		// sets an exponential backoff updater with a maximum backoff of 1 hour, and minimum of a second.
		minInterval := time.Second
		maxInterval := time.Hour // intentionally is set high to avoid overflow in this test.
		expUpdater := mempool.ExponentialUpdater(2, maxInterval, minInterval)
		chunks := 10
		expectedAttempts := 10

		withUpdaterScenario(t, chunks, expectedAttempts, expUpdater, func(t *testing.T, attempts uint64, lastTried time.Time,
			retryAfter time.Duration) {
			require.Equal(t, expectedAttempts, int(attempts)) // each chunk request should be attempted 10 times.

			// request should NOT be immediately qualified for retrial due to exponential backoff.
			require.True(t, !qualifier(attempts, lastTried, retryAfter))

			// retryAfter should be equal to 2^(attempts-1) * minInterval.
			// note that after the first attempt, retry after is set to minInterval.
			multiplier := time.Duration(math.Pow(2, float64(expectedAttempts-1)))
			expectedRetryAfter := minInterval * multiplier
			require.Equal(t, expectedRetryAfter, retryAfter)
		})
	})

	t.Run("10 chunks- 10 times exponential updater- underflow", func(t *testing.T) {
		// sets an exponential backoff updater with a maximum backoff of 1 hour, and minimum of a second.
		minInterval := time.Second
		maxInterval := time.Hour // intentionally is set high to avoid overflow in this test.
		// exponential multiplier is set to a very small number so that backoff always underflow, and set to
		// minInterval.
		expUpdater := mempool.ExponentialUpdater(0.001, maxInterval, minInterval)
		chunks := 10
		expectedAttempts := 10

		withUpdaterScenario(t, chunks, expectedAttempts, expUpdater, func(t *testing.T, attempts uint64, lastTried time.Time,
			retryAfter time.Duration) {
			require.Equal(t, expectedAttempts, int(attempts)) // each chunk request should be attempted 10 times.

			// request should NOT be immediately qualified for retrial due to exponential backoff.
			require.True(t, !qualifier(attempts, lastTried, retryAfter))

			// expected retry after should be equal to the min interval, since updates should always underflow due
			// to the very small multiplier.
			require.Equal(t, minInterval, retryAfter)
		})
	})

	t.Run("10 chunks- 10 times exponential updater- overflow", func(t *testing.T) {
		// sets an exponential backoff updater with a maximum backoff of 1 hour, and minimum of a second.
		minInterval := time.Second
		maxInterval := time.Minute
		// with exponential multiplier of 2, we expect to hit the overflow after 10 attempts.
		expUpdater := mempool.ExponentialUpdater(2, maxInterval, minInterval)
		chunks := 10
		expectedAttempts := 10

		withUpdaterScenario(t, chunks, expectedAttempts, expUpdater, func(t *testing.T, attempts uint64, lastTried time.Time,
			retryAfter time.Duration) {
			require.Equal(t, expectedAttempts, int(attempts)) // each chunk request should be attempted 10 times.

			// request should NOT be immediately qualified for retrial due to exponential backoff.
			require.True(t, !qualifier(attempts, lastTried, retryAfter))

			// expected retry after should be equal to the maxInterval, since updates should eventually overflow due
			// to the very small maxInterval and quite noticeable multiplier (2).
			require.Equal(t, maxInterval, retryAfter)
		})
	})
}

// withUpdaterScenario is a test helper that creates a chunk requests mempool and fills it with specified number of chunks.
// it then applies the updater on all of the chunks, and finally validates the chunks update history given the validator.
func withUpdaterScenario(t *testing.T, chunks int, times int, updater mempool.ChunkRequestHistoryUpdaterFunc,
	validate func(*testing.T, uint64, time.Time, time.Duration)) {

	// initializations: creating mempool and populating it.
	requests := stdmap.NewChunkRequests(uint(chunks))
	chunkReqs := unittest.ChunkDataPackRequestListFixture(chunks)
	for _, request := range chunkReqs {
		ok := requests.Add(request)
		require.True(t, ok)
	}

	// execution: updates request history of all chunks in mempool concurrently.
	wg := &sync.WaitGroup{}
	wg.Add(times * chunks)
	for _, request := range chunkReqs {
		for i := 0; i < times; i++ {
			go func(chunkID flow.Identifier) {
				_, _, _, ok := requests.UpdateRequestHistory(chunkID, updater)
				require.True(t, ok)

				wg.Done()
			}(request.ChunkID)
		}
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not finish updating requests on time")

	// performs custom validation of test.
	for _, request := range chunkReqs {
		attempts, lastTried, retryAfter, ok := requests.RequestHistory(request.ChunkID)
		require.True(t, ok)
		validate(t, attempts, lastTried, retryAfter)
	}
}

// TestFailingUpdater evaluates the atomicity of updating request history. If an update is failing, none of the history
// attributes of a request should be altered.
func TestFailingUpdater(t *testing.T) {
	// initializations: creating mempool and populating it, also updating each chunk request
	// with an incremental updater.
	requests := stdmap.NewChunkRequests(10)
	chunkReqs := unittest.ChunkDataPackRequestListFixture(10)
	for _, request := range chunkReqs {
		ok := requests.Add(request)
		require.True(t, ok)
	}

	wg := &sync.WaitGroup{}
	wg.Add(10)
	updater := mempool.IncrementalAttemptUpdater()
	for _, request := range chunkReqs {
		go func(chunkID flow.Identifier) {
			attempts, _, _, ok := requests.UpdateRequestHistory(chunkID, updater)
			require.True(t, ok)
			require.Equal(t, uint64(1), attempts)

			wg.Done()
		}(request.ChunkID)
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not finish updating requests on time")

	// execution and validation: updating request history of all chunks in mempool concurrently using
	// an updater that always failing should not change the requests' history
	failingUpdater := func(uint64, time.Duration) (uint64, time.Duration, bool) {
		return 0, 0, false
	}
	wg.Add(10)
	for _, request := range chunkReqs {
		go func(chunkID flow.Identifier) {
			// takes request history before update
			exAttempts, exLastTried, exRetryAfter, ok := requests.RequestHistory(chunkID)
			require.True(t, ok)

			// failing an update should not change request history
			_, _, _, result := requests.UpdateRequestHistory(chunkID, failingUpdater)
			require.False(t, result)

			acAttempts, acLastTried, acRetryAfter, ok := requests.RequestHistory(chunkID)
			require.True(t, ok)
			require.Equal(t, exAttempts, acAttempts)
			require.Equal(t, exLastTried, acLastTried)
			require.Equal(t, exRetryAfter, acRetryAfter)

			wg.Done()
		}(request.ChunkID)
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not finish updating requests on time")
}

// TestAddingDuplicateChunkIDs evaluates adding duplicate chunk ID requests
// that belong to an execution fork, i.e., same chunk ID appearing on two conflicting
// execution results.
func TestAddingDuplicateChunkIDs(t *testing.T) {
	// initializations: creating mempool and populating it.
	requests := stdmap.NewChunkRequests(10)

	thisReq := unittest.ChunkDataPackRequestFixture()
	require.True(t, requests.Add(thisReq))

	// adding another request for the same tuple of (chunkID, resultID, chunkIndex)
	// is deduplicated.
	require.False(t, requests.Add(&verification.ChunkDataPackRequest{
		Locator: chunks.Locator{
			ResultID: thisReq.ResultID,
			Index:    thisReq.Index,
		},
		ChunkDataPackRequestInfo: verification.ChunkDataPackRequestInfo{
			ChunkID: thisReq.ChunkID,
		},
	}))

	// adding another request for the same chunk ID but different result ID is stored.
	otherReq := &verification.ChunkDataPackRequest{
		Locator: chunks.Locator{
			ResultID: unittest.IdentifierFixture(),
			Index:    thisReq.Index,
		},
		ChunkDataPackRequestInfo: verification.ChunkDataPackRequestInfo{
			ChunkID:   thisReq.ChunkID,
			Agrees:    unittest.IdentifierListFixture(2),
			Disagrees: unittest.IdentifierListFixture(2),
		},
	}
	require.True(t, requests.Add(otherReq))

	// mempool size is based on unique chunk ids, and we only store one
	// chunk id.
	require.Equal(t, requests.Size(), uint(1))

	// All method must return request info, which is also bound by chunk id.
	reqInfoList := requests.All()
	require.Len(t, reqInfoList, 1)
	require.Equal(t, thisReq.ChunkID, reqInfoList[0].ChunkID)
	// agrees and disagrees must be union of all requests for that chunk ID.
	require.ElementsMatch(t, thisReq.Agrees.Union(otherReq.Agrees), reqInfoList[0].Agrees)
	require.ElementsMatch(t, thisReq.Disagrees.Union(otherReq.Disagrees), reqInfoList[0].Disagrees)

	locators, ok := requests.PopAll(thisReq.ChunkID)
	require.True(t, ok)
	require.True(t, locators.Contains(thisReq.Locator.ResultID, thisReq.Locator.Index))
	require.True(t, locators.Contains(otherReq.Locator.ResultID, otherReq.Locator.Index))

	// after poping all, mempool must be empty (since the requests for the only
	// chunk id have been poped).
	require.Equal(t, requests.Size(), uint(0))

	// PopAll on a non-existing chunk ID must return false
	locators, ok = requests.PopAll(thisReq.ChunkID)
	require.False(t, ok)
	require.Nil(t, locators)

}
