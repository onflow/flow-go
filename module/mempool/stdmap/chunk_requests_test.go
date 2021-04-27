package stdmap

import (
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
	increments := 5
	t.Run("5 times increment", func(t *testing.T) {

		// stores

		// updater
		incUpdater := mempool.IncrementalAttemptUpdater()

		// increments attempts
		for i := 0; i < increments; i++ {
			go func() {
				ok = requests.UpdateRequestHistory(request.ID(), incUpdater)
				require.True(t, ok)

				wg.Done()
			}()
		}

		unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not finish increments on time")

		// retrieves updated status
		expectedReq, ok := requests.ByID(request.ID())
		require.True(t, ok)
		require.Equal(t, request, expectedReq)
	})
}

func withUpdaterScenario(t *testing.T, chunks int, times int, updater mempool.ChunkRequestHistoryUpdaterFunc,
	validation func(t *testing.T, chunkIDs flow.IdentifierList, requests *ChunkRequests)) {

	// initializations: creating mempool and populating it.
	requests := NewChunkRequests(uint(chunks))
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
			go func() {
				ok := requests.UpdateRequestHistory(request.ID(), updater)
				require.True(t, ok)

				wg.Done()
			}()
		}
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not finish updating requests on time")

	// performs custom validation of test.
	validation(t, flow.GetIDs(chunkReqs), requests)
}
