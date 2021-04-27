package stdmap

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestIncrementStatus evaluates that calling increment attempt several times updates the status attempts.
func TestIncrementStatus(t *testing.T) {
	t.Run("10 chunks- 10 times incremental updater ", func(t *testing.T) {
		incUpdater := mempool.IncrementalAttemptUpdater()
		chunks := 10
		expectedAttempts := 10

		withUpdaterScenario(t, chunks, expectedAttempts, incUpdater, func(t *testing.T, attempts uint64, lastTried time.Time, retryAfter time.Duration) {
			require.Equal(t, expectedAttempts, int(attempts))             // each chunk request should be attempted 10 times.
			require.True(t, lastTried.Add(retryAfter).Before(time.Now())) // request should be immediately qualified for retrial.
		})
	})

	t.Run("10 chunks- 10 times exponential updater", func(t *testing.T) {
		interval := 2 * time.Second
		expUpdater := mempool.ExponentialBackoffWithCutoffUpdater(interval, 10*time.Hour)
		chunks := 10
		expectedAttempts := 10

		withUpdaterScenario(t, chunks, expectedAttempts, expUpdater, func(t *testing.T, attempts uint64, lastTried time.Time,
			retryAfter time.Duration) {
			require.Equal(t, expectedAttempts, int(attempts)) // each chunk request should be attempted 10 times.

			// request should NOT be immediately qualified for retrial due to exponential backoff.
			require.True(t, lastTried.Add(retryAfter).After(time.Now()))

			expectedRetryAfter := time.Duration(math.Pow(float64(interval), float64(expectedAttempts)) - 1)
			require.GreaterOrEqual(t, retryAfter, expectedRetryAfter)
		})
	})

}

func withUpdaterScenario(t *testing.T, chunks int, times int, updater mempool.ChunkRequestHistoryUpdaterFunc,
	validate func(*testing.T, uint64, time.Time, time.Duration)) {

	// initializations: creating mempool and populating it.
	requests := NewChunkRequests(uint(chunks))
	chunkReqs := unittest.ChunkDataPackRequestListFixture(chunks)
	fmt.Println(flow.GetIDs(chunkReqs))
	for _, request := range chunkReqs {
		ok := requests.Add(request)
		require.True(t, ok)
	}

	// execution: updates request history of all chunks in mempool concurrently.
	wg := &sync.WaitGroup{}
	wg.Add(times * chunks)
	for _, request := range chunkReqs {
		for i := 0; i < times; i++ {
			go func(requestID flow.Identifier) {
				ok := requests.UpdateRequestHistory(requestID, updater)
				require.True(t, ok)

				wg.Done()
			}(request.ID())
		}
	}
	wg.Wait()
	// unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not finish updating requests on time")

	// performs custom validation of test.
	for _, chunk := range chunkReqs {
		attempts, lastTried, retryAfter, ok := requests.RequestInfo(chunk.ID())
		require.True(t, ok)
		validate(t, attempts, lastTried, retryAfter)
	}
}
