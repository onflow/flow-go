package fetcher_test

import (
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockfetcher "github.com/onflow/flow-go/engine/verification/fetcher/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// FetcherEngineTestSuite encapsulates data structures for running unittests on fetcher engine.
type FetcherEngineTestSuite struct {
	// modules
	log                   zerolog.Logger
	metrics               *metrics.NoopCollector
	tracer                *trace.NoopTracer
	verifier              *mocknetwork.Engine               // the verifier engine
	state                 *protocol.State                   // used to verify the request origin
	pendingChunks         *mempool.ChunkStatuses            // used to store all the pending chunks that assigned to this node
	headers               *storage.Headers                  // used to fetch the block header when chunk data is ready to be verified
	chunkConsumerNotifier *module.ProcessingNotifier        // to report a chunk has been processed
	results               *storage.ExecutionResults         // to retrieve execution result of an assigned chunk
	receipts              *storage.ExecutionReceipts        // used to find executor of the chunk
	requester             *mockfetcher.ChunkDataPackHandler // used to request chunk data packs from network
}

// setupTest initiates a test suite prior to each test.
func setupTest() *FetcherEngineTestSuite {
	s := &FetcherEngineTestSuite{
		log:                   unittest.Logger(),
		metrics:               &metrics.NoopCollector{},
		tracer:                &trace.NoopTracer{},
		verifier:              &mocknetwork.Engine{},
		state:                 &protocol.State{},
		pendingChunks:         &mempool.ChunkStatuses{},
		headers:               &storage.Headers{},
		chunkConsumerNotifier: &module.ProcessingNotifier{},
		results:               &storage.ExecutionResults{},
		receipts:              &storage.ExecutionReceipts{},
		requester:             &mockfetcher.ChunkDataPackHandler{},
	}

	return s
}

// mockResultsByIDs mocks the results storage for affirmative querying of result IDs.
// Each result should be queried by the specified number of times.
func mockResultsByIDs(results *storage.ExecutionResults, list []*flow.ExecutionResult, times int) {
	for _, result := range list {
		results.On("ByID", result.ID()).Return(results).Times(times)
	}
}

// mockPendingChunksAdd mocks the add method of pending chunks for expecting only the specified list of chunk statuses.
// Each chunk status should be added only once.
// It should return the specified added boolean variable as the result of mocking.
func mockPendingChunksAdd(t *testing.T, pendingChunks *mempool.ChunkStatuses, list []*verification.ChunkStatus, added bool) {
	mu := &sync.Mutex{}

	pendingChunks.On("Add", mock.Anything).Run(func(args mock.Arguments) {
		// to provide mutual exclusion under concurrent invocations.
		mu.Lock()
		defer mu.Unlock()

		status, ok := args[0].(*verification.ChunkStatus)
		require.True(t, ok)

		// there should be a matching chunk status with the received one.
		statusID := status.ID()
		for _, s := range list {
			if s.Chunk.ID() == statusID {
				require.Equal(t, status.ExecutionResultID, s.ExecutionResultID)
				return
			}
		}

		require.Fail(t, "tried adding an unexpected chunk status to mempool")
	}).Return(added).Times(len(list))
}
