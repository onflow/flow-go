package requester_test

import (
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	mockfetcher "github.com/onflow/flow-go/engine/verification/fetcher/mock"
	"github.com/onflow/flow-go/engine/verification/requester"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/model/verification"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"

	protocol "github.com/onflow/flow-go/state/protocol/mock"
)

// RequesterEngineTestSuite encapsulates data structures for running unittests on requester engine.
type RequesterEngineTestSuite struct {
	// modules
	log             zerolog.Logger
	handler         *mockfetcher.ChunkDataPackHandler // contains callbacks for handling received chunk data packs.
	retryInterval   time.Duration                     // determines time in milliseconds for retrying chunk data requests.
	pendingRequests *mempool.ChunkRequests            // used to store all the pending chunks that assigned to this node
	state           *protocol.State                   // used to check the last sealed height
	con             *mocknetwork.Conduit              // used to send chunk data request, and receive the response

	// identities
	verIdentity *flow.Identity // verification node
}

// setupTest initiates a test suite prior to each test.
func setupTest() *RequesterEngineTestSuite {
	r := &RequesterEngineTestSuite{
		log:             unittest.Logger(),
		handler:         &mockfetcher.ChunkDataPackHandler{},
		retryInterval:   100 * time.Millisecond,
		pendingRequests: &mempool.ChunkRequests{},
		state:           &protocol.State{},
		verIdentity:     unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification)),
		con:             &mocknetwork.Conduit{},
	}

	return r
}

// newRequesterEngine returns a requester engine for testing.
func newRequesterEngine(t *testing.T, s *RequesterEngineTestSuite) *requester.Engine {
	net := &mock.Network{}
	// mocking the network registration of the engine
	net.On("Register", engine.RequestChunks, testifymock.Anything).
		Return(s.con, nil).
		Once()

	e, err := requester.New(s.log, s.state, net, s.retryInterval, s.pendingRequests, s.handler)
	require.NoError(t, err)
	testifymock.AssertExpectationsForObjects(t, net)

	return e
}

// TestHandleChunkDataPack_HappyPath evaluates the happy path of receiving a requested chunk data pack.
// The chunk data pack should be passed to the registered handler, and the resources should be cleaned up.
func TestHandleChunkDataPack_HappyPath(t *testing.T) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	response := unittest.ChunkDataResponseFixture(unittest.IdentifierFixture())
	originID := unittest.IdentifierFixture()

	// we have a request pending for this response chunk ID
	s.pendingRequests.On("ByID", response.ChunkDataPack.ChunkID).Return(&verification.ChunkRequestStatus{}, true).Once()
	// we remove pending request on receiving this response
	s.pendingRequests.On("Rem", response.ChunkDataPack.ChunkID).Return(true).Once()

	s.handler.On("HandleChunkDataPack", originID, &response.ChunkDataPack, &response.Collection).Return().Once()

	err := e.Process(originID, response)
	require.Nil(t, err)

	testifymock.AssertExpectationsForObjects(t, s.pendingRequests, s.con, s.handler)
}

// TestHandleChunkDataPack_HappyPath_Multiple evaluates the happy path of receiving several requested chunk data packs.
// Each chunk data pack should be handled once by being passed to the registered handler,
// the chunk ID and collection ID should match the response, and the resources should be cleaned up.
func TestHandleChunkDataPack_HappyPath_Multiple(t *testing.T) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	// creates list of chunk data pack responses
	count := 10
	responses := unittest.ChunkDataResponsesFixture(count)
	originID := unittest.IdentifierFixture()
	chunkCollectionIdMap := chunkToCollectionIdMap(t, responses)
	chunkIDs := toChunkIDs(chunkCollectionIdMap)

	// we have a request pending for this response chunk ID
	mockPendingRequestsByID(t, s.pendingRequests, chunkIDs)
	// we remove pending request on receiving this response
	mockPendingRequestsRem(t, s.pendingRequests, chunkIDs)
	// we pass each chunk data pack and its collection to chunk data pack handler
	mockChunkDataPackHandler(t, s.handler, chunkCollectionIdMap)

	for _, response := range responses {
		err := e.Process(originID, response)
		require.Nil(t, err)
	}
	testifymock.AssertExpectationsForObjects(t, s.pendingRequests, s.con, s.handler)
}

// TestHandleChunkDataPack_NonExistingRequest evaluates that receiving a chunk data pack response that does not have any request attached
// is dropped without passing it to the handler.
func TestHandleChunkDataPack_NonExistingRequest(t *testing.T) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	response := unittest.ChunkDataResponseFixture(unittest.IdentifierFixture())
	originID := unittest.IdentifierFixture()

	// we have a request pending for this response chunk ID
	s.pendingRequests.On("ByID", response.ChunkDataPack.ChunkID).Return(nil, false).Once()

	err := e.Process(originID, response)
	require.Nil(t, err)

	testifymock.AssertExpectationsForObjects(t, s.pendingRequests, s.con)
	s.handler.AssertNotCalled(t, "HandleChunkDataPack")
	s.pendingRequests.AssertNotCalled(t, "Rem")
}

// TestHandleChunkDataPack_NonExistingRequest evaluates that failing to remove a received chunk data pack's request
// from the memory terminates the procedure of handling a chunk data pack without passing it to the handler.
// The request for a chunk data pack may be removed from the memory if duplicate copies of a requested chunk data pack arrive
// concurrently. Then the mutex lock on pending requests mempool allows only one of those requested chunk data packs to remove the
// request and pass to handler. While handling the other ones gracefully terminated.
func TestHandleChunkDataPack_FailedRequestRemoval(t *testing.T) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	response := unittest.ChunkDataResponseFixture(unittest.IdentifierFixture())
	originID := unittest.IdentifierFixture()

	// we have a request pending for this response chunk ID
	s.pendingRequests.On("ByID", response.ChunkDataPack.ChunkID).Return(&verification.ChunkRequestStatus{}, true).Once()
	// however by the time we try remove it, the request status has gone.
	// this can happen when duplicate chunk data packs are coming concurrently.
	// the concurrency is safe with pending requests mempool's mutex lock.
	s.pendingRequests.On("Rem", response.ChunkDataPack.ChunkID).Return(false).Once()

	err := e.Process(originID, response)
	require.Nil(t, err)

	testifymock.AssertExpectationsForObjects(t, s.pendingRequests, s.con)
	s.handler.AssertNotCalled(t, "HandleChunkDataPack")
}

// TestRequestPendingChunkSealedBlock evaluates that requester engine drops pending requests for chunks belonging to
// sealed blocks, and also notifies the handler that this requested chunk has been sealed, so it no longer requests
// from the network it.
func TestRequestPendingChunkSealedBlock(t *testing.T) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	// creates a single chunk request status that belongs to a sealed height.
	aggrees := unittest.IdentifierListFixture(2)
	disaggrees := unittest.IdentifierListFixture(3)
	status := unittest.ChunkRequestStatusListFixture(1,
		unittest.WithHeight(5),
		unittest.WithAgrees(aggrees),
		unittest.WithDisagrees(disaggrees))
	mockLastSealedHeight(s.state, 10)
	s.pendingRequests.On("All").Return(status)
	mockPendingRequestsIncAttempt(t, s.pendingRequests, flow.GetIDs(status), 1)

	<-e.Ready()

	wg := mockPendingRequestsRem(t, s.pendingRequests, flow.GetIDs(status))
	unittest.RequireReturnsBefore(t, wg.Wait, time.Duration(2)*s.retryInterval, "could not request remove pending chunk on time")

	// requester does not call publish to disseminate the request for this chunk.
	s.con.AssertNotCalled(t, "Publish")
	<-e.Done()
}

// TestRequestPendingChunkDataPack evaluates happy path of having a single pending chunk requests.
// The chunk belongs to a non-sealed block.
// On timer interval, the chunk requests should be dispatched to the set of execution nodes agree with the execution
// result the chunk belongs to.
func TestRequestPendingChunkDataPack(t *testing.T) {
	testRequestPendingChunkDataPack(t, 1, 1)   // one request each one attempt
	testRequestPendingChunkDataPack(t, 10, 1)  // 10 requests each one attempt
	testRequestPendingChunkDataPack(t, 10, 10) // 10 requests each 10 attempts
}

// testRequestPendingChunkDataPack is a test helper that evaluates happy path of having a number of chunk requests pending.
// The test waits enough so that the required number of attempts is made on the chunks.
// The chunks belongs to a non-sealed block.
func testRequestPendingChunkDataPack(t *testing.T, requests int, attempts int) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	// creates 10 chunk request status each with 2 agree targets and 3 disagree targets.
	// chunk belongs to a block at heights greater than 5, but the last sealed block is at height 5, so
	// the chunk request should be dispatched.
	aggrees := unittest.IdentifierListFixture(2)
	disaggrees := unittest.IdentifierListFixture(3)
	status := unittest.ChunkRequestStatusListFixture(requests,
		unittest.WithHeightGreaterThan(5),
		unittest.WithAgrees(aggrees),
		unittest.WithDisagrees(disaggrees))
	mockLastSealedHeight(s.state, 5)
	s.pendingRequests.On("All").Return(status)
	mockPendingRequestsIncAttempt(t, s.pendingRequests, flow.GetIDs(status), attempts)

	<-e.Ready()

	wg := mockConduitForChunkDataPackRequest(t, s.con, status, attempts, func(response *messages.ChunkDataRequest) {})
	unittest.RequireReturnsBefore(t, wg.Wait, time.Duration(2*attempts)*s.retryInterval, "could not request and handle chunks on time")

	<-e.Done()
}

// chunkToCollectionIdMap is a test helper that extracts a chunkID -> collectionID map from chunk data responses.
func chunkToCollectionIdMap(t *testing.T, responses []*messages.ChunkDataResponse) map[flow.Identifier]flow.Identifier {
	chunkCollectionMap := make(map[flow.Identifier]flow.Identifier)
	for _, response := range responses {
		_, ok := chunkCollectionMap[response.ChunkDataPack.ChunkID]
		require.False(t, ok, "duplicate chunk ID found in fixture")
		chunkCollectionMap[response.ChunkDataPack.ChunkID] = response.ChunkDataPack.CollectionID
	}

	return chunkCollectionMap
}

// toChunkIDs is a test helper that extracts the chunk IDs from a chunk ID -> collection ID map.
func toChunkIDs(chunkToCollectionIDs map[flow.Identifier]flow.Identifier) flow.IdentifierList {
	chunkIDs := flow.IdentifierList{}
	for chunkID := range chunkToCollectionIDs {
		chunkIDs = append(chunkIDs, chunkID)
	}

	return chunkIDs
}

// mockConduitForChunkDataPackRequest mocks given conduit for requesting chunk data packs for given chunk IDs.
// Each chunk should be requested exactly `count` many time.
// Upon request, the given request handler is invoked.
// Also, the entire process should not exceed longer than the specified timeout.
func mockConduitForChunkDataPackRequest(t *testing.T,
	con *mocknetwork.Conduit,
	reqList []*verification.ChunkRequestStatus,
	count int,
	requestHandler func(response *messages.ChunkDataRequest)) *sync.WaitGroup {

	// counts number of requests for each chunk data pack
	reqCount := make(map[flow.Identifier]int)
	reqMap := make(map[flow.Identifier]*verification.ChunkRequestStatus)
	for _, status := range reqList {
		reqCount[status.ChunkID] = 0
		reqMap[status.ChunkID] = status
	}
	wg := &sync.WaitGroup{}

	// to counter race condition in concurrent invocations of Run
	mutex := &sync.Mutex{}
	wg.Add(count * len(reqList))

	con.On("Publish", testifymock.Anything, testifymock.Anything, testifymock.Anything).Run(func(args testifymock.Arguments) {
		mutex.Lock()
		defer mutex.Unlock()

		// requested chunk id from network should belong to list of chunk id requests the engine received.
		// also, it should not be repeated below a maximum threshold
		req, ok := args[0].(*messages.ChunkDataRequest)
		require.True(t, ok)
		require.Contains(t, flow.GetIDs(reqList), req.ChunkID)
		require.LessOrEqual(t, reqCount[req.ChunkID], count)
		reqCount[req.ChunkID]++

		// requested chunk ids should only be passed to agreed execution nodes
		target1, ok := args[1].(flow.Identifier)
		require.True(t, ok)
		require.Contains(t, reqMap[req.ChunkID].Agrees, target1)

		target2, ok := args[2].(flow.Identifier)
		require.True(t, ok)
		require.Contains(t, reqMap[req.ChunkID].Agrees, target2)

		go func() {
			requestHandler(req)
			wg.Done()
		}()

	}).Return(nil).Times(count * len(reqList)) // each chunk requested count time.

	return wg
}

// mockChunkDataPackHandler mocks chunk data pack handler for receiving a set of chunk ids.
// It evaluates that, each chunk ID should be passed only once accompanied with specified collection.
func mockChunkDataPackHandler(t *testing.T, handler *mockfetcher.ChunkDataPackHandler,
	chunkToCollectionIDs map[flow.Identifier]flow.Identifier) {
	chunkIDs := toChunkIDs(chunkToCollectionIDs)
	handledChunks := make(map[flow.Identifier]struct{})

	handler.On("HandleChunkDataPack", testifymock.Anything, testifymock.Anything, testifymock.Anything).Run(func(args testifymock.Arguments) {
		chunk, ok := args[1].(*flow.ChunkDataPack)
		require.True(t, ok)
		collection, ok := args[2].(*flow.Collection)
		require.True(t, ok)

		// we should have already requested this chunk data pack, and collection ID should be the same.
		chunkID := chunk.ID()
		require.Contains(t, chunkIDs, chunkID)
		require.Equal(t, chunkToCollectionIDs[chunkID], collection.ID())

		// invocation should be distinct per chunk ID
		_, ok = handledChunks[chunkID]
		require.False(t, ok)
		handledChunks[chunkID] = struct{}{}
	}).Return().Times(len(chunkIDs))
}

// mockPendingRequestsByID mocks chunk requests mempool for being queried for affirmative existence of each chunk ID once.
func mockPendingRequestsByID(t *testing.T, pendingRequests *mempool.ChunkRequests, chunkIDs flow.IdentifierList) {
	// maps keep track of distinct invocations per chunk ID
	retrievedRequests := make(map[flow.Identifier]struct{})

	// we have a request pending for this response chunk ID
	pendingRequests.On("ByID", testifymock.Anything).Run(func(args testifymock.Arguments) {
		chunkID, ok := args[0].(flow.Identifier)
		require.True(t, ok)
		// we should have already requested this chunk data pack
		require.Contains(t, chunkIDs, chunkID)

		// invocation should be distinct per chunk ID
		_, ok = retrievedRequests[chunkID]
		require.False(t, ok)
		retrievedRequests[chunkID] = struct{}{}
	}).Return(&verification.ChunkRequestStatus{}, true).
		Times(len(chunkIDs))
}

// mockPendingRequestsRem mocks chunk requests mempool for being queried for affirmative removal of each chunk ID once.
func mockPendingRequestsRem(t *testing.T, pendingRequests *mempool.ChunkRequests, chunkIDs flow.IdentifierList) *sync.WaitGroup {
	// maps keep track of distinct invocations per chunk ID
	removedRequests := make(map[flow.Identifier]struct{})

	wg := &sync.WaitGroup{}
	wg.Add(len(chunkIDs))
	// we remove pending request on receiving this response
	pendingRequests.On("Rem", testifymock.Anything).Run(func(args testifymock.Arguments) {
		chunkID, ok := args[0].(flow.Identifier)
		require.True(t, ok)
		// we should have already requested this chunk data pack
		require.Contains(t, chunkIDs, chunkID)

		// invocation should be distinct per chunk ID
		_, ok = removedRequests[chunkID]
		require.False(t, ok)
		removedRequests[chunkID] = struct{}{}

		wg.Done()
	}).
		Return(true).
		Times(len(chunkIDs))

	return wg
}

// mockPendingRequestsIncAttempt mocks chunk requests mempool for increasing the attempts on given chunk ids.
func mockPendingRequestsIncAttempt(t *testing.T, pendingRequests *mempool.ChunkRequests, chunkIDs flow.IdentifierList, attempts int) {
	pendingRequests.On("IncrementAttempt", testifymock.Anything).Run(func(args testifymock.Arguments) {
		chunkID, ok := args[0].(flow.Identifier)
		require.True(t, ok)
		// we should have already requested this chunk data pack
		require.Contains(t, chunkIDs, chunkID)

	}).
		Return(true).
		Times(len(chunkIDs) * attempts)

}

// mockLastSealedHeight mocks the protocol state for the specified last sealed height.
func mockLastSealedHeight(state *protocol.State, height uint64) {
	snapshot := &protocol.Snapshot{}
	header := unittest.BlockHeaderFixture()
	header.Height = height
	state.On("Sealed").Return(snapshot)
	snapshot.On("Head").Return(&header, nil)
}
