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

// mockReceiptsBlockID is a test helper that mocks the execution receipts mempool on ByBlockID method
// that returns two list of receipts for given block ID.
// First set of receipts are agree receipts, that have the same result ID as the given result.
// Second set of receipts are disagree receipts, that have a different result ID as the given result.
//
// It also returns the list of distinct executor node identities for all those receipts.
func mockReceiptsBlockID(t *testing.T,
	blockID flow.Identifier,
	receipts *storage.ExecutionReceipts,
	result *flow.ExecutionResult,
	agrees int,
	disagrees int) (flow.ExecutionReceiptList, flow.ExecutionReceiptList, flow.IdentityList, flow.IdentityList) {

	agreeReceipts := flow.ExecutionReceiptList{}
	disagreeReceipts := flow.ExecutionReceiptList{}
	agreeExecutors := flow.IdentityList{}
	disagreeExecutors := flow.IdentityList{}

	for i := 0; i < agrees; i++ {
		receipt := unittest.ExecutionReceiptFixture(unittest.WithResult(result))
		require.NotContains(t, agreeExecutors.NodeIDs(), receipt.ExecutorID)
		agreeExecutors = append(agreeExecutors, unittest.IdentityFixture(
			unittest.WithRole(flow.RoleExecution),
			unittest.WithNodeID(receipt.ExecutorID)))
		agreeReceipts = append(agreeReceipts, receipt)
	}

	for i := 0; i < disagrees; i++ {
		disagreeResult := unittest.ExecutionResultFixture()
		require.NotEqual(t, disagreeResult.ID(), result.ID())

		receipt := unittest.ExecutionReceiptFixture(unittest.WithResult(disagreeResult))
		require.NotContains(t, disagreeExecutors.NodeIDs(), receipt.ExecutorID)
		disagreeExecutors = append(disagreeExecutors, unittest.IdentityFixture(
			unittest.WithRole(flow.RoleExecution),
			unittest.WithNodeID(receipt.ExecutorID)))
		disagreeReceipts = append(disagreeReceipts, receipt)
	}

	all := append(agreeReceipts, disagreeReceipts...)

	receipts.On("ByBlockID", blockID).Return(all, nil)
	return agreeReceipts, disagreeReceipts, agreeExecutors, disagreeExecutors
}

// mockHeadersByBlockID is a test helper that mocks headers storage ByBlockID method for a header for given block ID
// at the given height.
func mockHeadersByBlockID(headers *storage.Headers, blockID flow.Identifier, height uint64) {
	header := unittest.BlockHeaderFixture()
	header.Height = height
	headers.On("ByBlockID", blockID).Return(&header, nil)
}

// mockStateAtBlockIDForExecutors is a test helper that mocks state at the block ID with the given execution nodes identities.
func mockStateAtBlockIDForExecutors(state *protocol.State, blockID flow.Identifier, executors flow.IdentityList) {
	snapshot := &protocol.Snapshot{}
	state.On("AtBlockID", blockID).Return(snapshot)
	snapshot.On("Identities", mock.Anything).Return(executors, nil)
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
