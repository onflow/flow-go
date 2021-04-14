package fetcher_test

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/model/chunks"
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
	block, result, statuses, locators := completeChunkStatusListFixture(t, 2, 1)
	mockBlockSealingStatus(s.state, s.headers, block.Header, false)
	mockResultsByIDs(s.results, []*flow.ExecutionResult{result})
	mockPendingChunksAdd(t, s.pendingChunks, statuses, true)
	mockPendingChunksRem(t, s.pendingChunks, statuses, true)
	mockPendingChunksByID(s.pendingChunks, statuses)

	_, _, agreeENs, _ := mockReceiptsBlockID(t, block.ID(), s.receipts, result, 1, 0)
	mockStateAtBlockIDForExecutors(s.state, block.ID(), agreeENs)
	requests := chunkRequestFixture(statuses.Chunks(), block.Header.Height, agreeENs.NodeIDs(), nil)

	chunkDataPacks, collections, verifiableChunks := verifiableChunkFixture(statuses.Chunks(), block, result)
	requesterWg := mockRequester(t, s.requester, requests, chunkDataPacks, collections, agreeENs, func(originID flow.Identifier,
		cdp *flow.ChunkDataPack,
		collection *flow.Collection) {
		e.HandleChunkDataPack(originID, cdp, collection)
	})

	verifierWG := mockVerifierEngine(t, s.verifier, verifiableChunks)
	mockChunkConsumerNotifier(t, s.chunkConsumerNotifier, flow.GetIDs(statuses.Chunks()))

	e.ProcessAssignedChunk(locators[0])

	unittest.RequireReturnsBefore(t, requesterWg.Wait, 1*time.Second, "could not handle received chunk data pack on time")
	unittest.RequireReturnsBefore(t, verifierWG.Wait, 1*time.Second, "could not push verifiable chunk on time")

	mock.AssertExpectationsForObjects(t, s.results)
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
	mockResultsByIDs(s.results, []*flow.ExecutionResult{result})

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

	mockResultsByIDs(s.results, []*flow.ExecutionResult{result})
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
func mockResultsByIDs(results *storage.ExecutionResults, list []*flow.ExecutionResult) {
	for _, result := range list {
		results.On("ByID", result.ID()).Return(result, nil)
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
	for _, id := range executors {
		snapshot.On("Identity", id.NodeID).Return(id, nil)
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

// mockPendingChunksRem mocks the remove method of pending chunks for expecting only the specified list of chunk statuses.
// Each chunk status should be removed only once.
// It should return the specified added boolean variable as the result of mocking.
func mockPendingChunksRem(t *testing.T, pendingChunks *mempool.ChunkStatuses, list []*verification.ChunkStatus, removed bool) {
	mu := &sync.Mutex{}

	pendingChunks.On("Rem", mock.Anything).Run(func(args mock.Arguments) {
		// to provide mutual exclusion under concurrent invocations.
		mu.Lock()
		defer mu.Unlock()

		actual, ok := args[0].(flow.Identifier)
		require.True(t, ok)

		// there should be a matching chunk status with the received one.
		for _, expected := range list {
			expectedID := expected.Chunk.ID()
			if bytes.Equal(expectedID[:], actual[:]) {
				return
			}
		}

		require.Fail(t, "tried removing an unexpected chunk status to mempool")
	}).Return(removed).Times(len(list))
}

// mockPendingChunksByID mocks the ByID method of pending chunks for expecting only the specified list of chunk statuses.
func mockPendingChunksByID(pendingChunks *mempool.ChunkStatuses, list []*verification.ChunkStatus) {
	mu := &sync.Mutex{}

	pendingChunks.On("ByID", mock.Anything).Return(
		func(chunkID flow.Identifier) *verification.ChunkStatus {
			// to provide mutual exclusion under concurrent invocations.
			mu.Lock()
			defer mu.Unlock()

			for _, expected := range list {
				expectedID := expected.Chunk.ID()
				if expectedID == chunkID {
					return expected
				}
			}
			return nil
		},
		func(chunkID flow.Identifier) bool {
			for _, expected := range list {
				expectedID := expected.Chunk.ID()
				if expectedID == chunkID {
					return true
				}
			}
			return false
		})
}

// mockVerifierEngine mocks verifier engine to expect receiving a matching chunk data pack with specified input.
// Each chunk data pack should be passed only once.
func mockVerifierEngine(t *testing.T,
	verifier *mocknetwork.Engine,
	verifiableChunks map[flow.Identifier]*verification.VerifiableChunkData) *sync.WaitGroup {
	mu := sync.Mutex{}
	wg := &sync.WaitGroup{}
	wg.Add(len(verifiableChunks))

	seen := make(map[flow.Identifier]struct{})

	verifier.On("ProcessLocal", mock.Anything).Run(func(args mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()

		vc, ok := args[0].(*verification.VerifiableChunkData)
		require.True(t, ok)

		// verifiable chunk data should be distinct.
		_, ok = seen[vc.Chunk.ID()]
		require.False(t, ok, "duplicated verifiable chunk received")
		seen[vc.Chunk.ID()] = struct{}{}

		// we should expect this verifiable chunk and its fields should match our expectation
		expected, ok := verifiableChunks[vc.Chunk.ID()]
		require.True(t, ok, "verifier engine received an unknown verifiable chunk data")

		require.Equal(t, *expected.ChunkDataPack, *vc.ChunkDataPack)
		require.Equal(t, expected.Collection.ID(), vc.Collection.ID())
		require.Equal(t, expected.Result.ID(), vc.Result.ID())
		require.Equal(t, expected.Header.ID(), vc.Header.ID())

		isSystemChunk := fetcher.IsSystemChunk(vc.Chunk.Index, vc.Result)
		require.Equal(t, isSystemChunk, vc.IsSystemChunk)

		// its end state should also match our expecation
		var endState flow.StateCommitment
		if isSystemChunk {
			// last chunk in a result is the system chunk and takes final state commitment
			var ok bool
			endState, ok = expected.Result.FinalStateCommitment()
			require.True(t, ok)
		} else {
			// any chunk except last takes the subsequent chunk's start state
			endState = expected.Result.Chunks[vc.Chunk.Index+1].StartState
		}

		require.Equal(t, endState, vc.EndState)
		wg.Done()
	}).Return(nil).Times(len(verifiableChunks))

	return wg
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

// verifiableChunkFixture is a test helper that creates verifiable chunks, chunk data packs,
// and collection fixtures for the given chunks list.
func verifiableChunkFixture(chunks flow.ChunkList, block *flow.Block, result *flow.ExecutionResult) (
	map[flow.Identifier]*flow.ChunkDataPack,
	map[flow.Identifier]*flow.Collection,
	map[flow.Identifier]*verification.VerifiableChunkData) {

	chunkDataPacks, collections := chunkDataPackResponseFixture(chunks)

	verifiableChunks := make(map[flow.Identifier]*verification.VerifiableChunkData)
	for _, chunk := range chunks {
		c := chunk // avoids shallow copy of loop variable
		chunkID := c.ID()
		verifiableChunks[chunkID] = &verification.VerifiableChunkData{
			Chunk:         c,
			Header:        block.Header,
			Result:        result,
			Collection:    collections[chunkID],
			ChunkDataPack: chunkDataPacks[chunkID],
		}
	}

	return chunkDataPacks, collections, verifiableChunks
}

// chunkRequestFixture is a test helper creates and returns chunk data pack requests for given chunks that all belong to the
// same block height.
// Agrees and disagrees are the list of execution node identifiers that generate the same and contradicting execution result
// with the execution result that chunks belong to, respectively.
func chunkRequestFixture(chunks flow.ChunkList, height uint64, agrees flow.IdentifierList, disagrees flow.IdentifierList) map[flow.Identifier]*verification.ChunkDataPackRequest {

	requests := make(map[flow.Identifier]*verification.ChunkDataPackRequest)
	for _, chunk := range chunks {
		requests[chunk.ID()] = &verification.ChunkDataPackRequest{
			ChunkID:   chunk.ID(),
			Height:    height,
			Agrees:    agrees,
			Disagrees: disagrees,
		}
	}

	return requests
}

// completeChunkStatusListFixture creates a reference block with an execution result associated with it.
// The result has specified number of chunks, which a random subset them are assumed assigned to fetcher engine,
// and hence have chunk status associated with them, i.e., `statusCount` of them.
//
// It returns the block, result, assigned chunk statuses, and their corresponding locators.
func completeChunkStatusListFixture(t *testing.T, chunkCount int, statusCount int) (*flow.Block, *flow.ExecutionResult, verification.ChunkStatusList,
	chunks.LocatorList) {
	require.LessOrEqual(t, statusCount, chunkCount)

	block := unittest.BlockFixture()
	result := unittest.ExecutionResultFixture(
		unittest.WithBlock(&block),
		unittest.WithChunks(uint(chunkCount)))
	statuses := unittest.ChunkStatusListFixture(t, []*flow.ExecutionResult{result}, statusCount)
	locators := unittest.ChunkStatusListToChunkLocatorFixture(statuses)

	return &block, result, statuses, locators
}
