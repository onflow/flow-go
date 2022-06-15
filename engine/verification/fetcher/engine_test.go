package fetcher_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/verification/fetcher"
	mockfetcher "github.com/onflow/flow-go/engine/verification/fetcher/mock"
	vertestutils "github.com/onflow/flow-go/engine/verification/utils/unittest"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	flowprotocol "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// FetcherEngineTestSuite encapsulates data structures for running unittests on fetcher engine.
type FetcherEngineTestSuite struct {
	log                   zerolog.Logger
	metrics               *module.VerificationMetrics
	tracer                *trace.NoopTracer
	verifier              *mocknetwork.Engine                 // the verifier engine
	state                 *protocol.State                     // used to verify the request origin
	pendingChunks         *mempool.ChunkStatuses              // used to store all the pending chunks that assigned to this node
	blocks                *storage.Blocks                     // used to for verifying collection ID.
	headers               *storage.Headers                    // used for building verifiable chunk data.
	chunkConsumerNotifier *module.ProcessingNotifier          // to report a chunk has been processed
	results               *storage.ExecutionResults           // to retrieve execution result of an assigned chunk
	receipts              *storage.ExecutionReceipts          // used to find executor of the chunk
	requester             *mockfetcher.ChunkDataPackRequester // used to request chunk data packs from network
}

// setupTest initiates a test suite prior to each test.
func setupTest() *FetcherEngineTestSuite {
	s := &FetcherEngineTestSuite{
		log:                   unittest.Logger(),
		metrics:               &module.VerificationMetrics{},
		tracer:                &trace.NoopTracer{},
		verifier:              &mocknetwork.Engine{},
		state:                 &protocol.State{},
		pendingChunks:         &mempool.ChunkStatuses{},
		headers:               &storage.Headers{},
		blocks:                &storage.Blocks{},
		chunkConsumerNotifier: &module.ProcessingNotifier{},
		results:               &storage.ExecutionResults{},
		receipts:              &storage.ExecutionReceipts{},
		requester:             &mockfetcher.ChunkDataPackRequester{},
	}

	return s
}

// newFetcherEngine returns a fetcher engine for testing.
func newFetcherEngine(s *FetcherEngineTestSuite) *fetcher.Engine {
	s.requester.On("WithChunkDataPackHandler", mock.AnythingOfType("*fetcher.Engine")).Return()

	e := fetcher.New(s.log,
		s.metrics,
		s.tracer,
		s.verifier,
		s.state,
		s.pendingChunks,
		s.headers,
		s.blocks,
		s.results,
		s.receipts,
		s.requester)

	e.WithChunkConsumerNotifier(s.chunkConsumerNotifier)
	return e
}

func TestProcessAssignedChunkHappyPath(t *testing.T) {
	tt := []struct {
		chunks   int
		assigned int
	}{
		{
			chunks:   1, // single chunk, single assigned
			assigned: 1,
		},
		{
			chunks:   2, // two chunks, single assigned
			assigned: 1,
		},
		{
			chunks:   4, // four chunks, two assigned
			assigned: 2,
		},
		{
			chunks:   10, // ten chunks, five assigned
			assigned: 5,
		},
	}

	for _, tc := range tt {
		t.Run(fmt.Sprintf("%d-chunk-%d-assigned", tc.chunks, tc.assigned), func(t *testing.T) {
			testProcessAssignChunkHappyPath(t, tc.chunks, tc.assigned)
		})
	}
}

// testProcessAssignChunkHappyPath evaluates behavior of fetcher engine respect to receiving some assigned chunks,
// it should request the requester a chunk data pack for each chunk.
// Then the test mocks sending a chunk data response for what fetcher engine requested.
// On receiving the response, fetcher engine should validate it and create and pass a verifiable chunk
// to the verifier engine.
// Once the verifier engine returns, the fetcher engine should notify the chunk consumer that it is done with
// this chunk.
func testProcessAssignChunkHappyPath(t *testing.T, chunkNum int, assignedNum int) {
	s := setupTest()
	e := newFetcherEngine(s)

	// creates a result with specified chunk number and assigned chunk numbers
	// also, the result has been created by two execution nodes, while the rest two have a conflicting result with it.
	block, result, statuses, locators, collMap := completeChunkStatusListFixture(t, chunkNum, assignedNum)
	_, _, agrees, disagrees := mockReceiptsBlockID(t, block.ID(), s.receipts, result, 2, 2)
	s.metrics.On("OnAssignedChunkReceivedAtFetcher").Return().Times(len(locators))

	// the chunks belong to an unsealed block.
	mockBlockSealingStatus(s.state, s.headers, block, false)

	// mocks resources on fetcher engine side.
	mockResultsByIDs(s.results, []*flow.ExecutionResult{result})
	mockBlocksStorage(s.blocks, s.headers, block)
	mockPendingChunksAdd(t, s.pendingChunks, statuses, true)
	mockPendingChunksRem(t, s.pendingChunks, statuses, true)
	mockPendingChunksGet(s.pendingChunks, statuses)
	mockStateAtBlockIDForIdentities(s.state, block.ID(), agrees.Union(disagrees))

	// generates and mocks requesting chunk data pack fixture
	requests := chunkRequestsFixture(result.ID(), statuses, agrees, disagrees)
	chunkDataPacks, verifiableChunks := verifiableChunksFixture(t, statuses, block, result, collMap)

	// fetcher engine should request chunk data for received (assigned) chunk locators
	s.metrics.On("OnChunkDataPackRequestSentByFetcher").Return().Times(len(requests))
	s.metrics.On("OnChunkDataPackArrivedAtFetcher").Return().Times(len(chunkDataPacks))
	requesterWg := mockRequester(t, s.requester, requests, chunkDataPacks,
		func(originID flow.Identifier, response *verification.ChunkDataPackResponse) {

			// mocks replying to the requests by sending a chunk data pack.
			e.HandleChunkDataPack(originID, response)
		})

	// fetcher engine should create and pass a verifiable chunk to verifier engine upon receiving each
	// chunk data responses, and notify the consumer that it is done with processing chunk.
	s.metrics.On("OnVerifiableChunkSentToVerifier").Return().Times(len(verifiableChunks))
	verifierWG := mockVerifierEngine(t, s.verifier, verifiableChunks)
	mockChunkConsumerNotifier(t, s.chunkConsumerNotifier, flow.GetIDs(locators.ToList()))

	// passes chunk data requests in parallel.
	processWG := &sync.WaitGroup{}
	processWG.Add(len(locators))
	for _, locator := range locators {
		go func(l *chunks.Locator) {
			e.ProcessAssignedChunk(l)
			processWG.Done()
		}(locator)
	}

	unittest.RequireReturnsBefore(t, requesterWg.Wait, 1*time.Second, "could not handle received chunk data pack on time")
	unittest.RequireReturnsBefore(t, verifierWG.Wait, 1*time.Second, "could not push verifiable chunk on time")
	unittest.RequireReturnsBefore(t, processWG.Wait, 1*time.Second, "could not process chunks on time")

	mock.AssertExpectationsForObjects(t, s.results, s.requester, s.pendingChunks, s.chunkConsumerNotifier, s.metrics)
}

// TestChunkResponse_RemovingStatusFails evaluates behavior of fetcher engine respect to receiving duplicate and concurrent
// chunk data pack responses to the same chunk.
// The deduplication of concurrent chunks happen at the pending chunk statuses mempool, that is protected by a mutex lock.
// So, of any concurrent chunk data pack responses of the same chunk, only one wins the lock and can successfully remove it
// from the mempool, while the others fail.
// If fetcher engine fails on removing a pending chunk status, it means that it has already been processed, and hence,
// it should drop it gracefully, without notifying the verifier or chunk consumer.
func TestChunkResponse_RemovingStatusFails(t *testing.T) {
	s := setupTest()
	e := newFetcherEngine(s)

	// creates a result with specified 2 chunks and a single assigned chunk to this fetcher engine.
	block, result, statuses, _, collMap := completeChunkStatusListFixture(t, 2, 1)
	_, _, agrees, _ := mockReceiptsBlockID(t, block.ID(), s.receipts, result, 2, 2)
	mockBlockSealingStatus(s.state, s.headers, block, false)

	mockResultsByIDs(s.results, []*flow.ExecutionResult{result})
	mockBlocksStorage(s.blocks, s.headers, block)
	mockPendingChunksGet(s.pendingChunks, statuses)
	mockStateAtBlockIDForIdentities(s.state, block.ID(), agrees)

	chunkLocatorID := statuses[0].ChunkLocatorID()
	// trying to remove the pending status fails.
	mockPendingChunksRem(t, s.pendingChunks, statuses, false)

	chunkDataPacks, _ := verifiableChunksFixture(t, statuses, block, result, collMap)

	s.metrics.On("OnChunkDataPackArrivedAtFetcher").Return().Once()
	e.HandleChunkDataPack(agrees[0].NodeID, chunkDataPacks[chunkLocatorID])

	// no verifiable chunk should be passed to verifier engine
	// and chunk consumer should not get any notification
	s.chunkConsumerNotifier.AssertNotCalled(t, "Notify")
	s.verifier.AssertNotCalled(t, "ProcessLocal")
	mock.AssertExpectationsForObjects(t, s.requester, s.pendingChunks, s.verifier, s.chunkConsumerNotifier, s.metrics)
}

// TestProcessAssignChunkSealedAfterRequest evaluates behavior of fetcher engine respect to receiving an assigned chunk
// that its block is getting sealed after requesting it from requester.
// The requester notifies the fetcher back that the block for chunk has been sealed.
// The fetcher engine then should remove chunk request status from memory, and notify the
// chunk consumer that it is done processing this chunk.
func TestProcessAssignChunkSealedAfterRequest(t *testing.T) {
	s := setupTest()
	e := newFetcherEngine(s)

	// creates a result with 2 chunks, which one of those chunks is assigned to this fetcher engine
	// also, the result has been created by two execution nodes, while the rest two have a conflicting result with it.
	// also the chunk belongs to an unsealed block.
	block, result, statuses, locators, collMap := completeChunkStatusListFixture(t, 2, 1)
	_, _, agrees, disagrees := mockReceiptsBlockID(t, block.ID(), s.receipts, result, 2, 2)
	mockBlockSealingStatus(s.state, s.headers, block, false)
	s.metrics.On("OnAssignedChunkReceivedAtFetcher").Return().Times(len(locators))

	// mocks resources on fetcher engine side.
	mockResultsByIDs(s.results, []*flow.ExecutionResult{result})
	mockPendingChunksAdd(t, s.pendingChunks, statuses, true)
	mockPendingChunksRem(t, s.pendingChunks, statuses, true)
	mockPendingChunksGet(s.pendingChunks, statuses)
	mockStateAtBlockIDForIdentities(s.state, block.ID(), agrees.Union(disagrees))

	// generates and mocks requesting chunk data pack fixture
	requests := chunkRequestsFixture(result.ID(), statuses, agrees, disagrees)
	responses, _ := verifiableChunksFixture(t, statuses, block, result, collMap)

	// fetcher engine should request chunk data for received (assigned) chunk locators
	// as the response it receives a notification that chunk belongs to a sealed block.
	// we mock this as the block is getting sealed after request dispatch.
	s.metrics.On("OnChunkDataPackRequestSentByFetcher").Return().Times(len(requests))
	requesterWg := mockRequester(t, s.requester, requests, responses, func(originID flow.Identifier,
		response *verification.ChunkDataPackResponse) {
		e.NotifyChunkDataPackSealed(response.Index, response.ResultID)
	})

	// fetcher engine should notify
	mockChunkConsumerNotifier(t, s.chunkConsumerNotifier, flow.GetIDs(locators.ToList()))

	// passes chunk data requests in parallel.
	processWG := &sync.WaitGroup{}
	processWG.Add(len(locators))
	for _, locator := range locators {
		go func(l *chunks.Locator) {
			e.ProcessAssignedChunk(l)
			processWG.Done()
		}(locator)
	}

	unittest.RequireReturnsBefore(t, requesterWg.Wait, time.Second, "could not handle sealed chunks notification on time")
	unittest.RequireReturnsBefore(t, processWG.Wait, 1*time.Second, "could not process chunks on time")

	mock.AssertExpectationsForObjects(t, s.requester, s.pendingChunks, s.chunkConsumerNotifier, s.metrics)
	// no verifiable chunk should be passed to verifier engine
	s.verifier.AssertNotCalled(t, "ProcessLocal")
}

// TestChunkResponse_InvalidChunkDataPack evaluates unhappy path of receiving an invalid chunk data response.
// A chunk data response is invalid if its integrity is violated. We consider collection id, chunk id, and start state,
// as the necessary conditions for chunk data integrity.
func TestChunkResponse_InvalidChunkDataPack(t *testing.T) {
	tt := []struct {
		alterChunkDataResponse func(*flow.ChunkDataPack)
		mockStateFunc          func(flow.Identity, *protocol.State, flow.Identifier) // mocks state at block identifier for the given identity.
		msg                    string
	}{
		{
			alterChunkDataResponse: func(cdp *flow.ChunkDataPack) {
				// re-writes collection with a random one that is different than original collection ID
				// in block's guarantee.
				txBody := unittest.TransactionBodyFixture()
				cdp.Collection.Transactions = []*flow.TransactionBody{
					&txBody,
				}
			},
			mockStateFunc: func(identity flow.Identity, state *protocol.State, blockID flow.Identifier) {
				// mocks a valid execution node as originID
				mockStateAtBlockIDForIdentities(state, blockID, flow.IdentityList{&identity})
			},
			msg: "conflicting-collection-with-blocks-storage",
		},
		{
			alterChunkDataResponse: func(cdp *flow.ChunkDataPack) {
				cdp.ChunkID = unittest.IdentifierFixture()
			},
			mockStateFunc: func(identity flow.Identity, state *protocol.State, blockID flow.Identifier) {
				// mocks a valid execution node as originID
				mockStateAtBlockIDForIdentities(state, blockID, flow.IdentityList{&identity})
			},
			msg: "invalid-chunk-ID",
		},
		{
			alterChunkDataResponse: func(cdp *flow.ChunkDataPack) {
				cdp.StartState = unittest.StateCommitmentFixture()
			},
			mockStateFunc: func(identity flow.Identity, state *protocol.State, blockID flow.Identifier) {
				// mocks a valid execution node as originID
				mockStateAtBlockIDForIdentities(state, blockID, flow.IdentityList{&identity})
			},
			msg: "invalid-start-state",
		},
		{
			alterChunkDataResponse: func(cdp *flow.ChunkDataPack) {
				// we don't alter chunk data pack content
			},
			mockStateFunc: func(identity flow.Identity, state *protocol.State, blockID flow.Identifier) {
				mockStateAtBlockIDForMissingIdentities(state, blockID, flow.IdentityList{&identity})
			},
			msg: "invalid-origin-id",
		},
		{
			alterChunkDataResponse: func(cdp *flow.ChunkDataPack) {
				// we don't alter chunk data pack content
			},
			mockStateFunc: func(identity flow.Identity, state *protocol.State, blockID flow.Identifier) {
				identity.Weight = 0
				mockStateAtBlockIDForIdentities(state, blockID, flow.IdentityList{&identity})
			},
			msg: "zero-weight-origin-id",
		},
		{
			alterChunkDataResponse: func(cdp *flow.ChunkDataPack) {
				// we don't alter chunk data pack content
			},
			mockStateFunc: func(identity flow.Identity, state *protocol.State, blockID flow.Identifier) {
				identity.Role = flow.RoleVerification
				mockStateAtBlockIDForIdentities(state, blockID, flow.IdentityList{&identity})
			},
			msg: "invalid-origin-role",
		},
	}

	for _, tc := range tt {
		t.Run(tc.msg, func(t *testing.T) {
			testInvalidChunkDataResponse(t, tc.alterChunkDataResponse, tc.mockStateFunc)
		})
	}
}

// testInvalidChunkDataResponse evaluates the unhappy path of receiving
// an invalid chunk data response for an already requested chunk.
// The invalid response should be dropped without any further action. Particularly, the
// notifier and verifier engine should not be called as the result of handling an invalid chunk.
//
// The input alter function alters the chunk data response to break its integrity.
func testInvalidChunkDataResponse(t *testing.T,
	alterChunkDataResponse func(*flow.ChunkDataPack),
	mockStateFunc func(flow.Identity, *protocol.State, flow.Identifier)) {
	s := setupTest()
	e := newFetcherEngine(s)

	// creates a result with 2 chunks, which one of those chunks is assigned to this fetcher engine
	// also, the result has been created by two execution nodes, while the rest two have a conflicting result with it.
	// also the chunk belongs to an unsealed block.
	block, result, statuses, _, collMap := completeChunkStatusListFixture(t, 2, 1)
	_, _, agrees, _ := mockReceiptsBlockID(t, block.ID(), s.receipts, result, 2, 2)

	// mocks resources on fetcher engine side.
	mockPendingChunksGet(s.pendingChunks, statuses)
	mockBlocksStorage(s.blocks, s.headers, block)

	chunkLocatorID := statuses[0].ChunkLocatorID()
	responses, _ := verifiableChunksFixture(t, statuses, block, result, collMap)

	// alters chunk data pack so that it become invalid.
	alterChunkDataResponse(responses[chunkLocatorID].Cdp)
	mockStateFunc(*agrees[0], s.state, block.ID())

	s.metrics.On("OnChunkDataPackArrivedAtFetcher").Return().Times(len(responses))
	e.HandleChunkDataPack(agrees[0].NodeID, responses[chunkLocatorID])

	mock.AssertExpectationsForObjects(t, s.pendingChunks, s.metrics)
	// no verifiable chunk should be passed to verifier engine
	// and chunk consumer should not get any notification
	s.chunkConsumerNotifier.AssertNotCalled(t, "Notify")
	s.verifier.AssertNotCalled(t, "ProcessLocal")

	// none of the subsequent calls on the pipeline path should happen upon validation fails.
	s.results.AssertNotCalled(t, "ByID")
	s.pendingChunks.AssertNotCalled(t, "Rem")
}

// TestChunkResponse_MissingStatus evaluates that if the fetcher engine receives a chunk data pack response for which
// it does not have any pending status, it drops it immediately and does not proceed handling pipeline.
// Receiving such chunk data response can happen in the following scenarios:
// - After requesting it to the network, requester informs fetcher engine that the chunk belongs to a sealed block.
// - More than one copy of the same response arrive at different time intervals, while the first copy has been handled.
func TestChunkResponse_MissingStatus(t *testing.T) {
	s := setupTest()
	e := newFetcherEngine(s)

	// creates a result with 2 chunks, which one of those chunks is assigned to this fetcher engine
	// also, the result has been created by two execution nodes, while the rest two have a conflicting result with it.
	// also the chunk belongs to an unsealed block.
	block, result, statuses, _, collMap := completeChunkStatusListFixture(t, 2, 1)
	status := statuses[0]
	responses, _ := verifiableChunksFixture(t, statuses, block, result, collMap)

	chunkLocatorID := statuses[0].ChunkLocatorID()

	// mocks there is no pending status for this chunk at fetcher engine.
	s.pendingChunks.On("Get", status.ChunkIndex, result.ID()).Return(nil, false)

	s.metrics.On("OnChunkDataPackArrivedAtFetcher").Return().Times(len(responses))
	e.HandleChunkDataPack(unittest.IdentifierFixture(), responses[chunkLocatorID])

	mock.AssertExpectationsForObjects(t, s.pendingChunks, s.metrics)

	// no verifiable chunk should be passed to verifier engine
	// and chunk consumer should not get any notification
	s.chunkConsumerNotifier.AssertNotCalled(t, "Notify")
	s.verifier.AssertNotCalled(t, "ProcessLocal")

	// none of the subsequent calls on the pipeline path should happen.
	s.results.AssertNotCalled(t, "ByID")
	s.blocks.AssertNotCalled(t, "ByID")
	s.pendingChunks.AssertNotCalled(t, "Rem")
	s.state.AssertNotCalled(t, "AtBlockID")
}

// TestSkipChunkOfSealedBlock evaluates that if fetcher engine receives a chunk belonging to a sealed block,
// it drops it without processing it any further and and notifies consumer
// that it is done with processing that chunk.
func TestSkipChunkOfSealedBlock(t *testing.T) {
	s := setupTest()
	e := newFetcherEngine(s)

	// creates a single chunk locator, and mocks its corresponding block sealed.
	block := unittest.BlockFixture()
	result := unittest.ExecutionResultFixture(unittest.WithExecutionResultBlockID(block.ID()))
	statuses := unittest.ChunkStatusListFixture(t, block.Header.Height, result, 1)
	locators := unittest.ChunkStatusListToChunkLocatorFixture(statuses)
	s.metrics.On("OnAssignedChunkReceivedAtFetcher").Return().Once()

	mockBlockSealingStatus(s.state, s.headers, &block, true)
	mockResultsByIDs(s.results, []*flow.ExecutionResult{result})

	// expects processing notifier being invoked upon sealed chunk detected,
	// which means the termination of processing a sealed chunk on fetcher engine
	// side.
	mockChunkConsumerNotifier(t, s.chunkConsumerNotifier, flow.GetIDs(locators.ToList()))

	for _, locator := range locators {
		e.ProcessAssignedChunk(locator)
	}

	mock.AssertExpectationsForObjects(t, s.results, s.metrics)
	// we should not request a duplicate chunk status.
	s.requester.AssertNotCalled(t, "Request")
	// we should not try adding a chunk of a sealed block to chunk status mempool.
	s.pendingChunks.AssertNotCalled(t, "Add")
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

// mockStateAtBlockIDForIdentities is a test helper that mocks state at the block ID with the given execution nodes identities.
func mockStateAtBlockIDForIdentities(state *protocol.State, blockID flow.Identifier, participants flow.IdentityList) {
	snapshot := &protocol.Snapshot{}
	state.On("AtBlockID", blockID).Return(snapshot)
	snapshot.On("Identities", mock.Anything).Return(participants, nil)
	for _, id := range participants {
		snapshot.On("Identity", id.NodeID).Return(id, nil)
	}
}

// mockStateAtBlockIDForMissingIdentities is a test helper that mocks state at the block ID with the given execution nodes identities as
// as missing ones, i.e., they are not part of the state.
func mockStateAtBlockIDForMissingIdentities(state *protocol.State, blockID flow.Identifier, participants flow.IdentityList) {
	snapshot := &protocol.Snapshot{}
	state.On("AtBlockID", blockID).Return(snapshot)
	for _, id := range participants {
		snapshot.On("Identity", id.NodeID).Return(nil, flowprotocol.IdentityNotFoundError{NodeID: id.NodeID})
	}
}

// mockPendingChunksAdd mocks the add method of pending chunks for expecting only the specified list of chunk statuses.
// Each chunk status should be added only once.
// It should return the specified added boolean variable as the result of mocking.
func mockPendingChunksAdd(t *testing.T, pendingChunks *mempool.ChunkStatuses, list []*verification.ChunkStatus, added bool) {
	mu := &sync.Mutex{}

	pendingChunks.On("Add", mock.Anything).
		Run(func(args mock.Arguments) {
			// to provide mutual exclusion under concurrent invocations.
			mu.Lock()
			defer mu.Unlock()

			actual, ok := args[0].(*verification.ChunkStatus)
			require.True(t, ok)

			// there should be a matching chunk status with the received one.
			actualLocatorID := actual.ChunkLocatorID()

			for _, expected := range list {
				expectedLocatorID := expected.ChunkLocatorID()
				if expectedLocatorID == actualLocatorID {
					require.Equal(t, expected.ExecutionResult, actual.ExecutionResult)
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

	pendingChunks.On("Rem", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			// to provide mutual exclusion under concurrent invocations.
			mu.Lock()
			defer mu.Unlock()

			actualIndex, ok := args[0].(uint64)
			require.True(t, ok)

			actualResultID, ok := args[1].(flow.Identifier)
			require.True(t, ok)

			// there should be a matching chunk status with the received one.
			for _, expected := range list {
				if expected.ChunkIndex == actualIndex && expected.ExecutionResult.ID() == actualResultID {
					return
				}
			}

			require.Fail(t, "tried removing an unexpected chunk status to mempool")
		}).Return(removed).Times(len(list))
}

// mockPendingChunksGet mocks the Get method of pending chunks for expecting only the specified list of chunk statuses.
func mockPendingChunksGet(pendingChunks *mempool.ChunkStatuses, list []*verification.ChunkStatus) {
	mu := &sync.Mutex{}

	pendingChunks.On("Get", mock.Anything, mock.Anything).Return(
		func(chunkIndex uint64, resultID flow.Identifier) *verification.ChunkStatus {
			// to provide mutual exclusion under concurrent invocations.
			mu.Lock()
			defer mu.Unlock()

			for _, expected := range list {
				if expected.ChunkIndex == chunkIndex && expected.ExecutionResult.ID() == resultID {
					return expected
				}
			}
			return nil
		},
		func(chunkIndex uint64, resultID flow.Identifier) bool {
			for _, expected := range list {
				if expected.ChunkIndex == chunkIndex && expected.ExecutionResult.ID() == resultID {
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

	verifier.On("ProcessLocal", mock.Anything).
		Run(func(args mock.Arguments) {
			mu.Lock()
			defer mu.Unlock()

			vc, ok := args[0].(*verification.VerifiableChunkData)
			require.True(t, ok)

			// verifiable chunk data should be distinct.
			_, ok = seen[chunks.ChunkLocatorID(vc.Result.ID(), vc.Chunk.Index)]
			require.False(t, ok, "duplicated verifiable chunk received")
			seen[chunks.ChunkLocatorID(vc.Result.ID(), vc.Chunk.Index)] = struct{}{}

			// we should expect this verifiable chunk and its fields should match our expectation
			expected, ok := verifiableChunks[chunks.ChunkLocatorID(vc.Result.ID(), vc.Chunk.Index)]
			require.True(t, ok, "verifier engine received an unknown verifiable chunk data")

			if vc.IsSystemChunk {
				// system chunk has an nil collection.
				require.Nil(t, vc.ChunkDataPack.Collection)
			} else {
				// non-system chunk has a non-nil collection.
				require.NotNil(t, vc.ChunkDataPack.Collection)
				require.Equal(t, expected.ChunkDataPack.Collection.ID(), vc.ChunkDataPack.Collection.ID())
			}

			require.Equal(t, *expected.ChunkDataPack, *vc.ChunkDataPack)
			require.Equal(t, expected.Result.ID(), vc.Result.ID())
			require.Equal(t, expected.Header.ID(), vc.Header.ID())

			isSystemChunk := fetcher.IsSystemChunk(vc.Chunk.Index, vc.Result)
			require.Equal(t, isSystemChunk, vc.IsSystemChunk)

			endState, err := fetcher.EndStateCommitment(vc.Result, vc.Chunk.Index, isSystemChunk)
			require.NoError(t, err)

			require.Equal(t, endState, vc.EndState)
			wg.Done()
		}).Return(nil)

	return wg
}

// mockChunkConsumerNotifier mocks the notify method of processing notifier to be notified exactly once per
// given chunk IDs.
func mockChunkConsumerNotifier(t *testing.T, notifier *module.ProcessingNotifier, locatorIDs flow.IdentifierList) {
	mu := &sync.Mutex{}
	seen := make(map[flow.Identifier]struct{})
	notifier.On("Notify", mock.Anything).Run(func(args mock.Arguments) {
		// to provide mutual exclusion under concurrent invocations.
		mu.Lock()
		defer mu.Unlock()

		locatorID, ok := args[0].(flow.Identifier)
		require.True(t, ok)
		require.Contains(t, locatorIDs, locatorID, "tried calling notifier on an unexpected locator ID")

		// each chunk should be notified once
		_, ok = seen[locatorID]
		require.False(t, ok)
		seen[locatorID] = struct{}{}

	}).Return().Times(len(locatorIDs))
}

// mockBlockSealingStatus mocks protocol state sealing status at height of given block.
func mockBlockSealingStatus(state *protocol.State, headers *storage.Headers, block *flow.Block, sealed bool) {
	headers.On("ByBlockID", block.ID()).Return(block.Header, nil)
	if sealed {
		vertestutils.MockLastSealedHeight(state, block.Header.Height+1)
	} else {
		vertestutils.MockLastSealedHeight(state, block.Header.Height-1)
	}
}

// mockBlocksStorage mocks blocks and headers storages for given block.
func mockBlocksStorage(blocks *storage.Blocks, headers *storage.Headers, block *flow.Block) {
	blockID := block.ID()
	blocks.On("ByID", blockID).Return(block, nil)
	headers.On("ByBlockID", blockID).Return(block.Header, nil)
}

// mockRequester mocks the chunk data pack requester with the given chunk data pack requests.
// Each chunk should be requested exactly once.
// On reply, it invokes the handler function with the given collection and chunk data pack for the chunk ID.
func mockRequester(t *testing.T,
	requester *mockfetcher.ChunkDataPackRequester,
	requests map[flow.Identifier]*verification.ChunkDataPackRequest,
	responses map[flow.Identifier]*verification.ChunkDataPackResponse,
	handler func(flow.Identifier, *verification.ChunkDataPackResponse)) *sync.WaitGroup {

	mu := sync.Mutex{}
	wg := &sync.WaitGroup{}
	wg.Add(len(requests))
	requester.On("Request", mock.Anything).
		Run(func(args mock.Arguments) {
			mu.Lock()
			defer mu.Unlock()

			actualRequest, ok := args[0].(*verification.ChunkDataPackRequest)
			require.True(t, ok)

			expectedRequest, ok := requests[actualRequest.ID()]
			require.True(t, ok, "requester received an unexpected chunk request")

			require.Equal(t, expectedRequest.Locator, actualRequest.Locator)
			require.Equal(t, expectedRequest.ChunkID, actualRequest.ChunkID)
			require.Equal(t, expectedRequest.Agrees, actualRequest.Agrees)
			require.Equal(t, expectedRequest.Disagrees, actualRequest.Disagrees)
			require.ElementsMatch(t, expectedRequest.Targets, actualRequest.Targets)

			go func() {
				response, ok := responses[actualRequest.ID()]
				require.True(t, ok)

				handler(actualRequest.Agrees[0], response)
				wg.Done()
			}()
		}).Return()

	return wg
}

// chunkDataPackResponsesFixture creates chunk data pack responses for given chunks.
func chunkDataPackResponsesFixture(t *testing.T,
	statuses verification.ChunkStatusList,
	collMap map[flow.Identifier]*flow.Collection,
	result *flow.ExecutionResult,
) map[flow.Identifier]*verification.ChunkDataPackResponse {
	responses := make(map[flow.Identifier]*verification.ChunkDataPackResponse)

	for _, status := range statuses {
		chunkLocatorID := status.ChunkLocatorID()
		responses[chunkLocatorID] = chunkDataPackResponseFixture(t, status.Chunk(), collMap[status.Chunk().ID()], result)
	}

	return responses
}

// chunkDataPackResponseFixture creates a chunk data pack response for given input.
func chunkDataPackResponseFixture(t *testing.T,
	chunk *flow.Chunk,
	collection *flow.Collection,
	result *flow.ExecutionResult) *verification.ChunkDataPackResponse {

	require.Equal(t, collection != nil, !fetcher.IsSystemChunk(chunk.Index, result), "only non-system chunks must have a collection")

	return &verification.ChunkDataPackResponse{
		Locator: chunks.Locator{
			ResultID: result.ID(),
			Index:    chunk.Index,
		},
		Cdp: unittest.ChunkDataPackFixture(chunk.ID(),
			unittest.WithStartState(chunk.StartState),
			unittest.WithChunkDataPackCollection(collection)),
	}
}

// verifiableChunksFixture is a test helper that creates verifiable chunks and chunk data responses.
func verifiableChunksFixture(t *testing.T,
	statuses verification.ChunkStatusList,
	block *flow.Block,
	result *flow.ExecutionResult,
	collMap map[flow.Identifier]*flow.Collection) (
	map[flow.Identifier]*verification.ChunkDataPackResponse,
	map[flow.Identifier]*verification.VerifiableChunkData) {

	responses := chunkDataPackResponsesFixture(t, statuses, collMap, result)

	verifiableChunks := make(map[flow.Identifier]*verification.VerifiableChunkData)
	for _, status := range statuses {
		chunkLocatorID := status.ChunkLocatorID()

		response, ok := responses[chunkLocatorID]
		require.True(t, ok, "missing chunk data response")

		verifiableChunks[chunkLocatorID] = verifiableChunkFixture(t, status.Chunk(), block, status.ExecutionResult, response.Cdp)
	}

	return responses, verifiableChunks
}

// verifiableChunksFixture is a test helper that creates verifiable chunks, chunk data packs,
// and collection fixtures for the given chunks list.
func verifiableChunkFixture(t *testing.T,
	chunk *flow.Chunk,
	block *flow.Block,
	result *flow.ExecutionResult,
	chunkDataPack *flow.ChunkDataPack) *verification.VerifiableChunkData {

	offsetForChunk, err := fetcher.TransactionOffsetForChunk(result.Chunks, chunk.Index)
	require.NoError(t, err)

	// TODO: add end state
	return &verification.VerifiableChunkData{
		Chunk:             chunk,
		Header:            block.Header,
		Result:            result,
		ChunkDataPack:     chunkDataPack,
		TransactionOffset: offsetForChunk,
	}
}

// chunkRequestsFixture is a test helper creates and returns chunk data pack requests for given result and chunk statuses.
// Agrees and disagrees are the list of execution node identifiers that generate the same and contradicting execution result
// with the execution result that chunks belong to, respectively.
func chunkRequestsFixture(
	resultID flow.Identifier,
	statuses verification.ChunkStatusList,
	agrees flow.IdentityList,
	disagrees flow.IdentityList) map[flow.Identifier]*verification.ChunkDataPackRequest {

	requests := make(map[flow.Identifier]*verification.ChunkDataPackRequest)
	for _, status := range statuses {
		chunkLocatorID := status.ChunkLocatorID()
		requests[chunkLocatorID] = chunkRequestFixture(resultID, status, agrees, disagrees)
	}

	return requests
}

// chunkRequestFixture creates and returns a chunk request for given result and chunk status.
//
// Agrees and disagrees are the list of execution node identifiers that generate the same and contradicting execution result
// with the execution result that chunks belong to, respectively.
func chunkRequestFixture(resultID flow.Identifier,
	status *verification.ChunkStatus,
	agrees flow.IdentityList,
	disagrees flow.IdentityList) *verification.ChunkDataPackRequest {

	return &verification.ChunkDataPackRequest{
		Locator: chunks.Locator{
			ResultID: resultID,
			Index:    status.ChunkIndex,
		},
		ChunkDataPackRequestInfo: verification.ChunkDataPackRequestInfo{
			ChunkID:   status.Chunk().ID(),
			Height:    status.BlockHeight,
			Agrees:    agrees.NodeIDs(),
			Disagrees: disagrees.NodeIDs(),
			Targets:   agrees.Union(disagrees),
		},
	}
}

// completeChunkStatusListFixture creates a reference block with an execution result associated with it.
// The result has specified number of chunks, which a random subset them are assumed assigned to fetcher engine,
// and hence have chunk status associated with them, i.e., `statusCount` of them.
//
// It returns the block, result, assigned chunk statuses, their corresponding locators, and a map between chunks to their collections.
func completeChunkStatusListFixture(t *testing.T, chunkCount int, statusCount int) (*flow.Block,
	*flow.ExecutionResult,
	verification.ChunkStatusList,
	chunks.LocatorMap,
	map[flow.Identifier]*flow.Collection) {
	require.LessOrEqual(t, statusCount, chunkCount)

	// keeps collections of assigned chunks
	collMap := make(map[flow.Identifier]*flow.Collection)

	collections := unittest.CollectionListFixture(chunkCount)

	block := unittest.BlockWithGuaranteesFixture(
		unittest.CollectionGuaranteesWithCollectionIDFixture(collections),
	)

	result := unittest.ExecutionResultFixture(
		unittest.WithBlock(block),
		unittest.WithChunks(uint(chunkCount)))
	statuses := unittest.ChunkStatusListFixture(t, block.Header.Height, result, statusCount)
	locators := unittest.ChunkStatusListToChunkLocatorFixture(statuses)

	for _, status := range statuses {
		if fetcher.IsSystemChunk(status.ChunkIndex, result) {
			// system-chunk should have a nil collection
			continue
		}
		collMap[status.Chunk().ID()] = collections[status.ChunkIndex]
	}

	return block, result, statuses, locators, collMap
}

func TestTransactionOffsetForChunk(t *testing.T) {
	t.Run("first chunk index always returns zero offset", func(t *testing.T) {
		offsetForChunk, err := fetcher.TransactionOffsetForChunk([]*flow.Chunk{nil}, 0)
		require.NoError(t, err)
		assert.Equal(t, uint32(0), offsetForChunk)
	})

	t.Run("offset is calculated", func(t *testing.T) {

		chunksList := []*flow.Chunk{
			{
				ChunkBody: flow.ChunkBody{
					NumberOfTransactions: 1,
				},
			},
			{
				ChunkBody: flow.ChunkBody{
					NumberOfTransactions: 2,
				},
			},
			{
				ChunkBody: flow.ChunkBody{
					NumberOfTransactions: 3,
				},
			},
			{
				ChunkBody: flow.ChunkBody{
					NumberOfTransactions: 5,
				},
			},
		}

		offsetForChunk, err := fetcher.TransactionOffsetForChunk(chunksList, 0)
		require.NoError(t, err)
		assert.Equal(t, uint32(0), offsetForChunk)

		offsetForChunk, err = fetcher.TransactionOffsetForChunk(chunksList, 1)
		require.NoError(t, err)
		assert.Equal(t, uint32(1), offsetForChunk)

		offsetForChunk, err = fetcher.TransactionOffsetForChunk(chunksList, 2)
		require.NoError(t, err)
		assert.Equal(t, uint32(3), offsetForChunk)

		offsetForChunk, err = fetcher.TransactionOffsetForChunk(chunksList, 3)
		require.NoError(t, err)
		assert.Equal(t, uint32(6), offsetForChunk)
	})

	t.Run("requesting index beyond length triggers error", func(t *testing.T) {

		chunksList := make([]*flow.Chunk, 2)

		_, err := fetcher.TransactionOffsetForChunk(chunksList, 2)
		require.Error(t, err)
	})
}
