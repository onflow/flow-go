package fetcher_test

import (
	"bytes"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/verification/fetcher"
	mockfetcher "github.com/onflow/flow-go/engine/verification/fetcher/mock"
	"github.com/onflow/flow-go/engine/verification/test"
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
	verifier              *mocknetwork.Engine                 // the verifier engine
	state                 *protocol.State                     // used to verify the request origin
	pendingChunks         *mempool.ChunkStatuses              // used to store all the pending chunks that assigned to this node
	headers               *storage.Headers                    // used to fetch the block header when chunk data is ready to be verified
	chunkConsumerNotifier *module.ProcessingNotifier          // to report a chunk has been processed
	results               *storage.ExecutionResults           // to retrieve execution result of an assigned chunk
	receipts              *storage.ExecutionReceipts          // used to find executor of the chunk
	requester             *mockfetcher.ChunkDataPackRequester // used to request chunk data packs from network
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
		requester:             &mockfetcher.ChunkDataPackRequester{},
	}

	return s
}

// newFetcherEngine returns a fetcher engine for testing.
func newFetcherEngine(s *FetcherEngineTestSuite) *fetcher.Engine {
	e := fetcher.New(s.log,
		s.metrics,
		s.tracer,
		s.verifier,
		s.state,
		s.pendingChunks,
		s.headers,
		s.results,
		s.receipts,
		s.requester)

	e.WithChunkConsumerNotifier(s.chunkConsumerNotifier)
	return e
}

// TestProcessAssignChunk_HappyPath evaluates behavior of fetcher engine respect to receiving a single assigned chunk,
// it should request the chunk data.
func TestProcessAssignChunk_HappyPath(t *testing.T) {
	s := setupTest()
	e := newFetcherEngine(s)

	// creates a single chunk locator, and mocks its corresponding block sealed.
	block := unittest.BlockFixture()
	result := unittest.ExecutionResultFixture(
		unittest.WithBlock(&block),
		unittest.WithChunks(2))
	statuses := unittest.ChunkStatusListFixture(t, []*flow.ExecutionResult{result}, 1)
	locators := unittest.ChunkStatusListToChunkLocatorFixture(statuses)
	mockBlockSealingStatus(s.state, s.headers, block.Header, false)
	mockResultsByIDs(s.results, []*flow.ExecutionResult{result}, 1)
	mockPendingChunksAdd(t, s.pendingChunks, statuses, true)

	_, _, agreeENs, _ := mockReceiptsBlockID(t, block.ID(), s.receipts, result, 1, 0)
	mockStateAtBlockIDForExecutors(s.state, block.ID(), agreeENs)
	requests := make(map[flow.Identifier]*verification.ChunkDataPackRequest)
	chunkID := result.Chunks[locators[0].Index].ID()
	requests[chunkID] = &verification.ChunkDataPackRequest{
		ChunkID: chunkID,
		Height:  block.Header.Height,
		Agrees:  agreeENs.NodeIDs(),
	}
	mockRequester(t, s.requester, requests, agreeENs)

	e.ProcessAssignedChunk(locators[0])

	mock.AssertExpectationsForObjects(t, s.results)
	// we should not request a duplicate chunk status.
	s.requester.AssertNotCalled(t, "Request")
	// we should not try adding a chunk of a sealed block to chunk status mempool.
	s.pendingChunks.AssertNotCalled(t, "Add")
}

// TestSkipChunkOfSealedBlock evaluates that if fetcher engine receives a chunk belonging to a sealed block,
// it drops it without processing it any further and and notifies consumer
// that it is done with processing that chunk.
func TestSkipChunkOfSealedBlock(t *testing.T) {
	s := setupTest()
	e := newFetcherEngine(s)

	// creates a single chunk locator, and mocks its corresponding block sealed.
	header := unittest.BlockHeaderFixture()
	result := unittest.ExecutionResultFixture(unittest.WithExecutionResultBlockID(header.ID()))
	statuses := unittest.ChunkStatusListFixture(t, []*flow.ExecutionResult{result}, 1)
	locators := unittest.ChunkStatusListToChunkLocatorFixture(statuses)
	mockBlockSealingStatus(s.state, s.headers, &header, true)
	mockResultsByIDs(s.results, []*flow.ExecutionResult{result}, 1)

	// expects processing notifier being invoked upon sealed chunk detected,
	// which means the termination of processing a sealed chunk on fetcher engine
	// side.
	mockChunkConsumerNotifier(t, s.chunkConsumerNotifier, flow.GetIDs(statuses))

	e.ProcessAssignedChunk(locators[0])

	mock.AssertExpectationsForObjects(t, s.results)
	// we should not request a duplicate chunk status.
	s.requester.AssertNotCalled(t, "Request")
	// we should not try adding a chunk of a sealed block to chunk status mempool.
	s.pendingChunks.AssertNotCalled(t, "Add")
}

// TestSkipDuplicateChunkStatus evaluates that if fetcher engine receives a duplicate chunk status
// for which it already has a pending chunk status in memory, it drops the duplicate and notifies consumer
// that it is done with processing that chunk.
//
// Note that fetcher engine relies on chunk consumer to perform the deduplication, and provide distinct chunk
// locators. So, this test evaluates a rare unhappy path that its occurrence would indicate a data race.
func TestSkipDuplicateChunkStatus(t *testing.T) {
	s := setupTest()
	e := newFetcherEngine(s)

	// creates a single chunk locator, and mocks its corresponding block unsealed.
	header := unittest.BlockHeaderFixture()
	result := unittest.ExecutionResultFixture(unittest.WithExecutionResultBlockID(header.ID()))
	statuses := unittest.ChunkStatusListFixture(t, []*flow.ExecutionResult{result}, 1)
	locators := unittest.ChunkStatusListToChunkLocatorFixture(statuses)
	mockBlockSealingStatus(s.state, s.headers, &header, false)

	mockResultsByIDs(s.results, []*flow.ExecutionResult{result}, 1)
	// mocks duplicate chunk exists on pending chunks, i.e., returning false on adding
	// same locators.
	mockPendingChunksAdd(t, s.pendingChunks, statuses, false)

	// expects processing notifier being invoked upon deduplication detected,
	// which means the termination of processing a duplicate chunk on fetcher engine
	// side.
	mockChunkConsumerNotifier(t, s.chunkConsumerNotifier, flow.GetIDs(statuses))

	e.ProcessAssignedChunk(locators[0])

	mock.AssertExpectationsForObjects(t, s.pendingChunks, s.results)
	// we should not request a duplicate chunk status.
	s.requester.AssertNotCalled(t, "Request")
}

// mockResultsByIDs mocks the results storage for affirmative querying of result IDs.
// Each result should be queried by the specified number of times.
func mockResultsByIDs(results *storage.ExecutionResults, list []*flow.ExecutionResult, times int) {
	for _, result := range list {
		results.On("ByID", result.ID()).Return(result, nil).Times(times)
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
		require.NotContains(t, agreeExecutors.NodeIDs(), receipt.ExecutorID) // should not have duplicate executors
		agreeExecutors = append(agreeExecutors, unittest.IdentityFixture(
			unittest.WithRole(flow.RoleExecution),
			unittest.WithNodeID(receipt.ExecutorID)))
		agreeReceipts = append(agreeReceipts, receipt)
	}

	for i := 0; i < disagrees; i++ {
		disagreeResult := unittest.ExecutionResultFixture()
		require.NotEqual(t, disagreeResult.ID(), result.ID())

		receipt := unittest.ExecutionReceiptFixture(unittest.WithResult(disagreeResult))
		require.NotContains(t, agreeExecutors.NodeIDs(), receipt.ExecutorID)    // should not have an executor in both lists
		require.NotContains(t, disagreeExecutors.NodeIDs(), receipt.ExecutorID) // should not have duplicate executors
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

		actual, ok := args[0].(*verification.ChunkStatus)
		require.True(t, ok)

		// there should be a matching chunk status with the received one.
		statusID := actual.Chunk.ID()

		for _, expected := range list {
			expectedID := expected.Chunk.ID()
			if bytes.Equal(expectedID[:], statusID[:]) {
				require.Equal(t, expected.ExecutionResultID, actual.ExecutionResultID)
				return
			}
		}

		require.Fail(t, "tried adding an unexpected chunk status to mempool")
	}).Return(added).Times(len(list))
}

// mockChunkConsumerNotifier mocks the notify method of processing notifier to be notified exactly once per
// given chunk IDs.
func mockChunkConsumerNotifier(t *testing.T, notifier *module.ProcessingNotifier, chunkIDs flow.IdentifierList) {
	mu := &sync.Mutex{}
	seen := make(map[flow.Identifier]struct{})
	notifier.On("Notify", mock.Anything).Run(func(args mock.Arguments) {
		// to provide mutual exclusion under concurrent invocations.
		mu.Lock()
		defer mu.Unlock()

		chunkID, ok := args[0].(flow.Identifier)
		require.True(t, ok)
		require.Contains(t, chunkIDs, chunkID, "tried calling notifier on an unexpected chunk ID")

		// each chunk should be notified once
		_, ok = seen[chunkID]
		require.False(t, ok)
		seen[chunkID] = struct{}{}

	}).Return().Times(len(chunkIDs))
}

// mockBlockSealingStatus mocks protocol state sealing status at height of given block header.
func mockBlockSealingStatus(state *protocol.State, headers *storage.Headers, header *flow.Header, sealed bool) {
	headers.On("ByBlockID", header.ID()).Return(header, nil)
	if sealed {
		test.MockLastSealedHeight(state, header.Height+1)
	} else {
		test.MockLastSealedHeight(state, header.Height-1)
	}
}

// mockRequester mocks the chunk data pack requester with the given chunk data pack requests.
// Each chunk should be requested exactly once.
// On reply, it invokes the handler function with the given collection and chunk data pack for the chunk ID.
func mockRequester(t *testing.T, requester *mockfetcher.ChunkDataPackRequester,
	requests map[flow.Identifier]*verification.ChunkDataPackRequest,
	chunkDataPacks map[flow.Identifier]*flow.ChunkDataPack,
	collections map[flow.Identifier]*flow.Collection,
	allExecutors flow.IdentityList, handler func(flow.Identifier, *flow.ChunkDataPack, *flow.Collection)) *sync.WaitGroup {

	mu := sync.Mutex{}
	wg := &sync.WaitGroup{}
	wg.Add(len(requests))
	requester.On("Request", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()

		actualRequest, ok := args[0].(*verification.ChunkDataPackRequest)
		require.True(t, ok)

		expectedRequest, ok := requests[actualRequest.ChunkID]
		require.True(t, ok, "requester received an unexpected chunk request")

		require.Equal(t, *expectedRequest, *actualRequest)

		actualExecutors, ok := args[1].(flow.IdentityList)
		require.True(t, ok)
		require.ElementsMatchf(t, allExecutors, actualExecutors, "execution nodes lists do not match")

		go func() {
			cdp, ok := chunkDataPacks[actualRequest.ChunkID]
			require.True(t, ok)

			collection, ok := collections[actualRequest.ChunkID]
			require.True(t, ok)

			handler(actualRequest.Agrees[0], cdp, collection)
			wg.Done()
		}()
	}).Return().Times(len(requests))

	return wg
}

// chunkDataPackResponseFixture creates chunk data pack and collections for given chunks.
func chunkDataPackResponseFixture(chunks flow.ChunkList) (map[flow.Identifier]*flow.ChunkDataPack,
	map[flow.Identifier]*flow.Collection) {
	chunkDataPacks := make(map[flow.Identifier]*flow.ChunkDataPack)
	collections := make(map[flow.Identifier]*flow.Collection)

	for _, chunk := range chunks {
		chunkID := chunk.ID()
		collection := unittest.CollectionFixture(1)
		collections[chunkID] = &collection
		chunkDataPacks[chunkID] = unittest.ChunkDataPackFixture(chunkID,
			unittest.WithStartState(chunk.StartState),
			unittest.WithCollectionID(collection.ID()))
	}

	return chunkDataPacks, collections
}
