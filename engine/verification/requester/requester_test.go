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
	vertestutils "github.com/onflow/flow-go/engine/verification/utils/unittest"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module"
	flowmempool "github.com/onflow/flow-go/module/mempool"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// RequesterEngineTestSuite encapsulates data structures for running unittests on requester engine.
type RequesterEngineTestSuite struct {
	// modules
	log             zerolog.Logger
	handler         *mockfetcher.ChunkDataPackHandler // contains callbacks for handling received chunk data packs.
	pendingRequests *mempool.ChunkRequests            // used to store all the pending chunks that assigned to this node
	state           *protocol.State                   // used to check the last sealed height
	con             *mocknetwork.Conduit              // used to send chunk data request, and receive the response
	tracer          module.Tracer
	metrics         *mock.VerificationMetrics

	// identities
	verIdentity *flow.Identity // verification node

	// parameters
	requestTargets uint64
	retryInterval  time.Duration // determines time in milliseconds for retrying chunk data requests.
}

// setupTest initiates a test suite prior to each test.
func setupTest() *RequesterEngineTestSuite {
	r := &RequesterEngineTestSuite{
		log:             unittest.Logger(),
		tracer:          &trace.NoopTracer{},
		metrics:         &mock.VerificationMetrics{},
		handler:         &mockfetcher.ChunkDataPackHandler{},
		retryInterval:   100 * time.Millisecond,
		requestTargets:  2,
		pendingRequests: &mempool.ChunkRequests{},
		state:           &protocol.State{},
		verIdentity:     unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification)),
		con:             &mocknetwork.Conduit{},
	}

	return r
}

// newRequesterEngine returns a requester engine for testing.
func newRequesterEngine(t *testing.T, s *RequesterEngineTestSuite) *requester.Engine {
	net := &mocknetwork.Network{}
	// mocking the network registration of the engine
	net.On("Register", engine.RequestChunks, testifymock.Anything).
		Return(s.con, nil).
		Once()

	e, err := requester.New(s.log,
		s.state,
		net,
		s.tracer,
		s.metrics,
		s.pendingRequests,
		s.retryInterval,
		// requests are only qualified if their retryAfter is elapsed.
		requester.RetryAfterQualifier,
		// exponential backoff with multiplier of 2, minimum interval of a second, and
		// maximum interval of an hour.
		flowmempool.ExponentialUpdater(2, time.Hour, time.Second),
		s.requestTargets)
	require.NoError(t, err)
	testifymock.AssertExpectationsForObjects(t, net)

	e.WithChunkDataPackHandler(s.handler)

	return e
}

// TestHandleChunkDataPack_HappyPath evaluates the happy path of receiving a requested chunk data pack.
// The chunk data pack should be passed to the registered handler, and the resources should be cleaned up.
func TestHandleChunkDataPack_HappyPath(t *testing.T) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	response := unittest.ChunkDataResponseMsgFixture(unittest.IdentifierFixture())
	request := unittest.ChunkDataPackRequestFixture(unittest.WithChunkID(response.ChunkDataPack.ChunkID))
	originID := unittest.IdentifierFixture()

	// we remove pending request on receiving this response
	s.pendingRequests.On("PopAll", response.ChunkDataPack.ChunkID).Return(request, true).Once()

	s.handler.On("HandleChunkDataPack", originID, &verification.ChunkDataPackResponse{
		Locator: chunks.Locator{
			ResultID: request.ResultID,
			Index:    request.Index,
		},
		Cdp: &response.ChunkDataPack,
	}).Return().Once()
	s.metrics.On("OnChunkDataPackResponseReceivedFromNetworkByRequester").Return().Once()
	s.metrics.On("OnChunkDataPackSentToFetcher").Return().Once()

	err := e.Process(engine.RequestChunks, originID, response)
	require.Nil(t, err)

	testifymock.AssertExpectationsForObjects(t, s.con, s.handler, s.pendingRequests, s.metrics)
}

// TestHandleChunkDataPack_HappyPath_Multiple evaluates the happy path of receiving several requested chunk data packs.
// Each chunk data pack should be handled once by being passed to the registered handler,
// the chunk ID and collection ID should match the response, and the resources should be cleaned up.
func TestHandleChunkDataPack_HappyPath_Multiple(t *testing.T) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	// creates list of chunk data pack responses
	count := 10
	responses := unittest.ChunkDataResponseMessageListFixture(count)
	originID := unittest.IdentifierFixture()
	chunkIDs := toChunkIDs(t, responses)

	// we remove pending request on receiving this response
	mockPendingRequestsRem(t, s.pendingRequests, chunkIDs)
	// we pass each chunk data pack and its collection to chunk data pack handler
	mockChunkDataPackHandler(t, s.handler, chunkIDs)
	s.metrics.On("OnChunkDataPackResponseReceivedFromNetworkByRequester").Return().Times(len(responses))
	s.metrics.On("OnChunkDataPackSentToFetcher").Return().Times(len(responses))

	for _, response := range responses {
		err := e.Process(engine.RequestChunks, originID, response)
		require.Nil(t, err)
	}
	testifymock.AssertExpectationsForObjects(t, s.pendingRequests, s.con, s.handler, s.metrics)
}

// TestHandleChunkDataPack_NonExistingRequest evaluates that failing to remove a received chunk data pack's request
// from the memory terminates the procedure of handling a chunk data pack without passing it to the handler.
// The request for a chunk data pack may be removed from the memory if duplicate copies of a requested chunk data pack arrive
// concurrently. Then the mutex lock on pending requests mempool allows only one of those requested chunk data packs to remove the
// request and pass to handler. While handling the other ones gracefully terminated.
func TestHandleChunkDataPack_FailedRequestRemoval(t *testing.T) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	response := unittest.ChunkDataResponseMsgFixture(unittest.IdentifierFixture())
	originID := unittest.IdentifierFixture()

	// however by the time we try remove it, the request has gone.
	// this can happen when duplicate chunk data packs are coming concurrently.
	// the concurrency is safe with pending requests mempool's mutex lock.
	s.pendingRequests.On("Rem", response.ChunkDataPack.ChunkID).Return(false).Once()
	s.metrics.On("OnChunkDataPackResponseReceivedFromNetworkByRequester").Return().Once()

	err := e.Process(engine.RequestChunks, originID, response)
	require.Nil(t, err)

	testifymock.AssertExpectationsForObjects(t, s.pendingRequests, s.con, s.metrics)
	s.handler.AssertNotCalled(t, "HandleChunkDataPack")
}

// TestRequestPendingChunkSealedBlock evaluates that requester engine drops pending requests for chunks belonging to
// sealed blocks, and also notifies the handler that this requested chunk has been sealed, so it no longer requests
// from the network it.
func TestRequestPendingChunkSealedBlock(t *testing.T) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	// creates a single chunk request that belongs to a sealed height.
	agrees := unittest.IdentifierListFixture(2)
	disagrees := unittest.IdentifierListFixture(3)
	requests := unittest.ChunkDataPackRequestListFixture(1,
		unittest.WithHeight(5),
		unittest.WithAgrees(agrees),
		unittest.WithDisagrees(disagrees))
	vertestutils.MockLastSealedHeight(s.state, 10)
	s.pendingRequests.On("All").Return(requests)
	// check data pack request is never tried since its block has been sealed.
	s.metrics.On("SetMaxChunkDataPackAttemptsForNextUnsealedHeightAtRequester", uint64(0)).Return().Once()

	unittest.RequireCloseBefore(t, e.Ready(), time.Second, "could not start engine on time")

	mockPendingRequestsRem(t, s.pendingRequests, flow.GetIDs(requests))
	notifierWG := mockNotifyBlockSealedHandler(t, s.handler, flow.GetIDs(requests))
	unittest.RequireReturnsBefore(t, notifierWG.Wait, time.Duration(2)*s.retryInterval, "could not notify the handler on time")

	// requester does not call publish to disseminate the request for this chunk.
	s.con.AssertNotCalled(t, "Publish")
	unittest.RequireCloseBefore(t, e.Done(), time.Second, "could not stop engine on time")
}

// TestCompleteRequestingUnsealedChunkCycle evaluates a complete life cycle of receiving a chunk request by the requester.
// The requester should submit the request to the network (on its timer overflow), and receive the response back and send it to
// the registered handler.
//
// It should also clean the request from memory.
func TestCompleteRequestingUnsealedChunkLifeCycle(t *testing.T) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	sealedHeight := uint64(10)
	// Creates a single chunk request with its corresponding response.
	// The chunk belongs to an unsealed block.
	agrees := unittest.IdentifierListFixture(2)
	disagrees := unittest.IdentifierListFixture(3)
	requests := unittest.ChunkDataPackRequestListFixture(1,
		unittest.WithHeightGreaterThan(sealedHeight),
		unittest.WithAgrees(agrees),
		unittest.WithDisagrees(disagrees))
	response := unittest.ChunkDataResponseMsgFixture(requests[0].ChunkID)

	// mocks the requester pipeline
	vertestutils.MockLastSealedHeight(s.state, sealedHeight)
	s.pendingRequests.On("All").Return(requests)
	mockChunkDataPackHandler(t, s.handler, flow.IdentifierList{response.ChunkDataPack.ChunkID})
	mockPendingRequestsRem(t, s.pendingRequests, flow.GetIDs(requests))

	// makes all chunk requests being qualified for dispatch instantly
	qualifyWG := mockPendingRequestInfoAndUpdate(t,
		s.pendingRequests, flow.GetIDs(requests), flow.IdentifierList{}, flow.IdentifierList{}, 1)
	s.metrics.On("OnChunkDataPackResponseReceivedFromNetworkByRequester").Return().Times(len(requests))
	s.metrics.On("OnChunkDataPackRequestDispatchedInNetworkByRequester").Return().Times(len(requests))
	s.metrics.On("OnChunkDataPackSentToFetcher").Return().Times(len(requests))
	s.metrics.On("SetMaxChunkDataPackAttemptsForNextUnsealedHeightAtRequester", uint64(1)).Return().Once()

	unittest.RequireCloseBefore(t, e.Ready(), time.Second, "could not start engine on time")

	// we wait till the engine submits the chunk request to the network, and receive the response
	conduitWG := mockConduitForChunkDataPackRequest(t, s.con, requests, 1, func(request *messages.ChunkDataRequest) {
		err := e.Process(engine.RequestChunks, requests[0].Agrees[0], response)
		require.NoError(t, err)
	})
	unittest.RequireReturnsBefore(t, qualifyWG.Wait, time.Duration(2)*s.retryInterval, "could not check chunk requests qualification on time")
	unittest.RequireReturnsBefore(t, conduitWG.Wait, time.Duration(2)*s.retryInterval, "could not request chunks from network")

	unittest.RequireCloseBefore(t, e.Done(), time.Second, "could not stop engine on time")
	testifymock.AssertExpectationsForObjects(t, s.pendingRequests, s.metrics)
}

// TestRequestPendingChunkSealedBlock_Hybrid evaluates the situation that requester has some pending chunk requests belonging to sealed blocks
// (i.e., sealed chunks), and some pending chunk requests belonging to unsealed blocks (i.e., unsealed chunks).
//
// On timer, the requester should submit pending requests for unsealed chunks to the network, while dropping the requests for the
// sealed chunks, and notify the handler.
func TestRequestPendingChunkSealedBlock_Hybrid(t *testing.T) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	sealedHeight := uint64(10)
	// creates 2 chunk data packs that belong to a sealed height, and
	// 3 that belong to an unsealed height.
	agrees := unittest.IdentifierListFixture(2)
	disagrees := unittest.IdentifierListFixture(3)
	sealedRequests := unittest.ChunkDataPackRequestListFixture(2,
		unittest.WithHeight(sealedHeight-1),
		unittest.WithAgrees(agrees),
		unittest.WithDisagrees(disagrees))
	unsealedRequests := unittest.ChunkDataPackRequestListFixture(3,
		unittest.WithHeightGreaterThan(sealedHeight),
		unittest.WithAgrees(agrees),
		unittest.WithDisagrees(disagrees))
	requests := append(sealedRequests, unsealedRequests...)

	vertestutils.MockLastSealedHeight(s.state, sealedHeight)
	s.pendingRequests.On("All").Return(requests)

	// makes all (unsealed) chunk requests being qualified for dispatch instantly
	qualifyWG := mockPendingRequestInfoAndUpdate(t,
		s.pendingRequests, flow.GetIDs(unsealedRequests), flow.IdentifierList{}, flow.IdentifierList{}, 1)
	s.metrics.On("OnChunkDataPackRequestDispatchedInNetworkByRequester").Return().Times(len(unsealedRequests))
	// each unsealed height is requested only once, hence the maximum is updated only once from 0 -> 1
	s.metrics.On("SetMaxChunkDataPackAttemptsForNextUnsealedHeightAtRequester", testifymock.Anything).Return().Once()

	unittest.RequireCloseBefore(t, e.Ready(), time.Second, "could not start engine on time")

	// sealed requests should be removed and the handler should be notified.
	mockPendingRequestsRem(t, s.pendingRequests, flow.GetIDs(sealedRequests))
	notifierWG := mockNotifyBlockSealedHandler(t, s.handler, flow.GetIDs(sealedRequests))
	// unsealed requests should be submitted to the network once
	conduitWG := mockConduitForChunkDataPackRequest(t, s.con, unsealedRequests, 1, func(*messages.ChunkDataRequest) {})

	unittest.RequireReturnsBefore(t, qualifyWG.Wait, time.Duration(2)*s.retryInterval, "could not check chunk requests qualification on time")
	unittest.RequireReturnsBefore(t, notifierWG.Wait, time.Duration(2)*s.retryInterval, "could not notify the handler on time")
	unittest.RequireReturnsBefore(t, conduitWG.Wait, time.Duration(2)*s.retryInterval, "could not request chunks from network")
	unittest.RequireCloseBefore(t, e.Done(), time.Second, "could not stop engine on time")

	testifymock.AssertExpectationsForObjects(t, s.pendingRequests, s.metrics)
}

func TestHandleChunkDataPack_DuplicateChunkIDs(t *testing.T) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	resultA, resultB, _, _ := vertestutils.ExecutionResultForkFixture(t)

	responseA := unittest.ChunkDataResponseMsgFixture(resultA.Chunks[0].ID())
	responseB := unittest.ChunkDataResponseMsgFixture(resultB.Chunks[0].ID())
	requestA := unittest.ChunkDataPackRequestFixture(unittest.WithChunkID(responseA.ChunkDataPack.ChunkID))
	requestB := unittest.ChunkDataPackRequestFixture(unittest.WithChunkID(responseB.ChunkDataPack.ChunkID))

	originID := unittest.IdentifierFixture()

	// we remove pending request on receiving this response
	s.pendingRequests.On("PopAll", responseA.ChunkDataPack.ChunkID).Return(requestA, true).Once()
	s.pendingRequests.On("PopAll", responseB.ChunkDataPack.ChunkID).Return(requestB, true).Once()

	s.handler.On("HandleChunkDataPack", originID, &verification.ChunkDataPackResponse{
		Locator: chunks.Locator{
			ResultID: requestA.ResultID,
			Index:    requestA.Index,
		},
		Cdp: &responseA.ChunkDataPack,
	}).Return().Once()

	s.handler.On("HandleChunkDataPack", originID, &verification.ChunkDataPackResponse{
		Locator: chunks.Locator{
			ResultID: requestB.ResultID,
			Index:    requestB.Index,
		},
		Cdp: &responseB.ChunkDataPack,
	}).Return().Once()

	s.metrics.On("OnChunkDataPackResponseReceivedFromNetworkByRequester").Return().Twice()
	s.metrics.On("OnChunkDataPackSentToFetcher").Return().Twice()

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		err := e.Process(engine.RequestChunks, originID, responseA)
		require.Nil(t, err)

		wg.Done()
	}()

	go func() {
		err := e.Process(engine.RequestChunks, originID, responseB)
		require.Nil(t, err)

		wg.Done()
	}()

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "could not process chunk responses on time")
	testifymock.AssertExpectationsForObjects(t, s.con, s.handler, s.pendingRequests, s.metrics)
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
func testRequestPendingChunkDataPack(t *testing.T, count int, attempts int) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	// creates 10 chunk request each with 2 agree targets and 3 disagree targets.
	// chunk belongs to a block at heights greater than 5, but the last sealed block is at height 5, so
	// the chunk request should be dispatched.
	agrees := unittest.IdentifierListFixture(2)
	disagrees := unittest.IdentifierListFixture(3)
	requests := unittest.ChunkDataPackRequestListFixture(count,
		unittest.WithHeightGreaterThan(5),
		unittest.WithAgrees(agrees),
		unittest.WithDisagrees(disagrees))
	vertestutils.MockLastSealedHeight(s.state, 5)
	s.pendingRequests.On("All").Return(requests)

	// makes all chunk requests being qualified for dispatch instantly
	qualifyWG := mockPendingRequestInfoAndUpdate(t,
		s.pendingRequests, flow.GetIDs(requests), flow.IdentifierList{}, flow.IdentifierList{}, attempts)

	s.metrics.On("OnChunkDataPackRequestDispatchedInNetworkByRequester").Return().Times(count * attempts)
	s.metrics.On("SetMaxChunkDataPackAttemptsForNextUnsealedHeightAtRequester", testifymock.Anything).Run(func(args testifymock.Arguments) {
		actualAttempts, ok := args[0].(uint64)
		require.True(t, ok)

		require.LessOrEqual(t, actualAttempts, uint64(attempts))
	}).Return().Times(attempts)

	unittest.RequireCloseBefore(t, e.Ready(), time.Second, "could not start engine on time")

	conduitWG := mockConduitForChunkDataPackRequest(t, s.con, requests, attempts, func(*messages.ChunkDataRequest) {})
	unittest.RequireReturnsBefore(t, qualifyWG.Wait, time.Duration(2*attempts)*s.retryInterval,
		"could not check chunk requests qualification on time")
	unittest.RequireReturnsBefore(t, conduitWG.Wait, time.Duration(2*attempts)*s.retryInterval, "could not request and handle chunks on time")

	unittest.RequireCloseBefore(t, e.Done(), time.Second, "could not stop engine on time")
	testifymock.AssertExpectationsForObjects(t, s.pendingRequests, s.metrics)
}

// TestDispatchingRequests_Hybrid evaluates the behavior of requester when it has different request dispatch timelines, i.e.,
// some requests should be dispatched instantly to the network. Some others are old and planned for late dispatch (out of this test timeline),
// and some other should not be dispatched since they no longer are needed (and will be cleaned on next iteration).
//
// The test evaluates that only requests that are instantly planned are getting dispatched to the network.
func TestDispatchingRequests_Hybrid(t *testing.T) {
	s := setupTest()
	e := newRequesterEngine(t, s)

	// Generates 30 requests, 10 of each type.
	//
	// requests belong to the chunks of
	// a block at heights greater than 5, but the last sealed block is at height 5, so
	// the chunk request should be dispatched.
	agrees := unittest.IdentifierListFixture(2)
	disagrees := unittest.IdentifierListFixture(3)
	vertestutils.MockLastSealedHeight(s.state, 5)
	// models requests that are just added to the mempool and are ready to dispatch.
	instantQualifiedRequests := unittest.ChunkDataPackRequestListFixture(10,
		unittest.WithHeightGreaterThan(5),
		unittest.WithAgrees(agrees),
		unittest.WithDisagrees(disagrees))
	// models old requests that stayed long in the mempool and are not dispatched anytime soon.
	lateQualifiedRequests := unittest.ChunkDataPackRequestListFixture(10,
		unittest.WithHeightGreaterThan(5),
		unittest.WithAgrees(agrees),
		unittest.WithDisagrees(disagrees))
	// models requests that their chunk data pack arrives during the dispatch processing and hence
	// are no longer needed to dispatch.
	disQualifiedRequests := unittest.ChunkDataPackRequestListFixture(10,
		unittest.WithHeightGreaterThan(5),
		unittest.WithAgrees(agrees),
		unittest.WithDisagrees(disagrees))

	allRequests := append(instantQualifiedRequests, lateQualifiedRequests...)
	allRequests = append(allRequests, disQualifiedRequests...)
	s.pendingRequests.On("All").Return(allRequests)

	attempts := 10 // waits for 10 iterations of onTimer cycle in requester.
	qualifyWG := mockPendingRequestInfoAndUpdate(t,
		s.pendingRequests,
		flow.GetIDs(instantQualifiedRequests),
		flow.GetIDs(lateQualifiedRequests),
		flow.GetIDs(disQualifiedRequests),
		attempts)

	unittest.RequireCloseBefore(t, e.Ready(), time.Second, "could not start engine on time")

	// mocks only instantly qualified requests are dispatched in the network.
	conduitWG := mockConduitForChunkDataPackRequest(t, s.con, instantQualifiedRequests, attempts, func(*messages.ChunkDataRequest) {})
	s.metrics.On("OnChunkDataPackRequestDispatchedInNetworkByRequester").Return().Times(len(instantQualifiedRequests) * attempts)
	// each instantly qualified one is requested only once, hence the maximum is updated only once from 0 -> 1, and
	// is kept at 1 during all cycles of this test.
	s.metrics.On("SetMaxChunkDataPackAttemptsForNextUnsealedHeightAtRequester", uint64(1)).Return()

	unittest.RequireReturnsBefore(t, qualifyWG.Wait, time.Duration(2*attempts)*s.retryInterval,
		"could not check chunk requests qualification on time")
	unittest.RequireReturnsBefore(t, conduitWG.Wait, time.Duration(2*attempts)*s.retryInterval,
		"could not request and handle chunks on time")
	unittest.RequireCloseBefore(t, e.Done(), time.Second, "could not stop engine on time")

	testifymock.AssertExpectationsForObjects(t, s.pendingRequests, s.metrics)
}

// toChunkIDs is a test helper that extracts chunk ids from chunk data pack responses.
func toChunkIDs(t *testing.T, responses []*messages.ChunkDataResponse) flow.IdentifierList {
	var chunkIDs flow.IdentifierList
	for _, response := range responses {
		require.NotContains(t, chunkIDs, response.ChunkDataPack.ChunkID, "duplicate chunk ID found in fixture")
		chunkIDs = append(chunkIDs, response.ChunkDataPack.ChunkID)
	}
	return chunkIDs
}

// mockConduitForChunkDataPackRequest mocks given conduit for requesting chunk data packs for given chunk IDs.
// Each chunk should be requested exactly `count` many time.
// Upon request, the given request handler is invoked.
// Also, the entire process should not exceed longer than the specified timeout.
func mockConduitForChunkDataPackRequest(t *testing.T,
	con *mocknetwork.Conduit,
	reqList verification.ChunkDataPackRequestList,
	count int,
	requestHandler func(*messages.ChunkDataRequest)) *sync.WaitGroup {

	// counts number of requests for each chunk data pack
	reqCount := make(map[flow.Identifier]int)
	reqMap := make(map[flow.Identifier]*verification.ChunkDataPackRequest)
	for _, request := range reqList {
		reqCount[request.ChunkID] = 0
		reqMap[request.ChunkID] = request
	}
	wg := &sync.WaitGroup{}

	// to counter race condition in concurrent invocations of Run
	mutex := &sync.Mutex{}
	wg.Add(count * len(reqList))

	con.On("Publish", testifymock.Anything, testifymock.Anything, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
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

		}).Return(nil)

	return wg
}

// mockChunkDataPackHandler mocks chunk data pack handler for receiving a set of chunk ids.
// It evaluates that, each chunk ID should be passed only once accompanied with specified collection.
func mockChunkDataPackHandler(t *testing.T, handler *mockfetcher.ChunkDataPackHandler, chunkIDs flow.IdentifierList) {
	handledChunks := make(map[flow.Identifier]struct{})

	handler.On("HandleChunkDataPack", testifymock.Anything, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			_, ok := args[0].(flow.Identifier)
			require.True(t, ok)
			chunk, ok := args[1].(*flow.ChunkDataPack)
			require.True(t, ok)

			// we should have already requested this chunk data pack.
			chunkID := chunk.ID()
			require.Contains(t, chunkIDs, chunkID)

			// invocation should be distinct per chunk ID
			_, ok = handledChunks[chunkID]
			require.False(t, ok)
			handledChunks[chunkID] = struct{}{}
		}).Return().Times(len(chunkIDs))
}

// mockChunkDataPackHandler mocks chunk data pack handler for being notified that a set of chunk IDs are sealed.
// It evaluates that, each chunk ID should be notified only once.
func mockNotifyBlockSealedHandler(t *testing.T, handler *mockfetcher.ChunkDataPackHandler,
	chunkIDs flow.IdentifierList) *sync.WaitGroup {

	wg := &sync.WaitGroup{}
	wg.Add(len(chunkIDs))
	sealedChunks := make(map[flow.Identifier]struct{})
	handler.On("NotifyChunkDataPackSealed", testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			chunkID, ok := args[0].(flow.Identifier)
			require.True(t, ok)

			// we should have already requested this chunk data pack, and collection ID should be the same.
			require.Contains(t, chunkIDs, chunkID)

			// invocation should be distinct per chunk ID
			_, ok = sealedChunks[chunkID]
			require.False(t, ok)
			sealedChunks[chunkID] = struct{}{}

			wg.Done()
		}).Return()

	return wg
}

// mockPendingRequestsRem mocks chunk requests mempool for being queried for affirmative removal of each chunk ID once.
func mockPendingRequestsRem(t *testing.T, pendingRequests *mempool.ChunkRequests, chunkIDs flow.IdentifierList) {
	// maps keep track of distinct invocations per chunk ID
	removedRequests := make(map[flow.Identifier]struct{})

	// we remove pending request on receiving this response
	pendingRequests.On("Rem", testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			chunkID, ok := args[0].(flow.Identifier)
			require.True(t, ok)
			// we should have already requested this chunk data pack
			require.Contains(t, chunkIDs, chunkID)

			// invocation should be distinct per chunk ID
			_, ok = removedRequests[chunkID]
			require.False(t, ok)
			removedRequests[chunkID] = struct{}{}
		}).
		Return(true).
		Times(len(chunkIDs))
}

// mockPendingRequestInfoAndUpdate mocks pending requests mempool regarding three sets of chunk IDs: the instant, late, and disqualified ones.
// The chunk IDs in the instantly qualified requests will be instantly qualified for dispatching in the networking layer.
// The chunk IDs in the late qualified requests will be postponed to a very later time for dispatching. The postponed time is set so long
// that they literally never get the chance to dispatch within the test time, e.g., 1 hour.
// The chunk IDs in the disqualified requests do not dispatch at all.
//
// The disqualified ones represent the set of chunk requests that are cleaned from memory during the on timer iteration of the requester
// engine, and are no longer needed.
func mockPendingRequestInfoAndUpdate(t *testing.T,
	pendingRequests *mempool.ChunkRequests,
	instantQualifiedReqs flow.IdentifierList,
	lateQualifiedReqs flow.IdentifierList,
	disQualifiedReqs flow.IdentifierList,
	attempts int) *sync.WaitGroup {

	wg := &sync.WaitGroup{}

	// for purpose of test and due to having a mocked mempool, we assume disqualified requests reside on the
	// mempool, so their qualification is getting checked on each attempt iteration (and rejected).
	total := attempts * (len(instantQualifiedReqs) + len(lateQualifiedReqs) + len(disQualifiedReqs))
	wg.Add(total)

	pendingRequests.On("RequestHistory", testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			// type assertion of input.
			chunkID, ok := args[0].(flow.Identifier)
			require.True(t, ok)

			// chunk ID should be one of the expected ones.
			require.True(t,
				instantQualifiedReqs.Contains(chunkID) ||
					lateQualifiedReqs.Contains(chunkID) ||
					disQualifiedReqs.Contains(chunkID))

			wg.Done()

		}).Return(
		// number of attempts
		func(chunkID flow.Identifier) uint64 {
			if instantQualifiedReqs.Contains(chunkID) || lateQualifiedReqs.Contains(chunkID) {
				return uint64(1)
			}

			return uint64(0)

		}, // last tried timestamp
		func(chunkID flow.Identifier) time.Time {
			if instantQualifiedReqs.Contains(chunkID) {
				// mocks last tried long enough so they instantly get qualified.
				return time.Now().Add(-1 * time.Hour)
			}

			if lateQualifiedReqs.Contains(chunkID) {
				return time.Now()
			}

			return time.Time{}
		}, // retry after duration
		func(chunkID flow.Identifier) time.Duration {
			if instantQualifiedReqs.Contains(chunkID) {
				// mocks retry after very short so they instantly get qualified.
				return 1 * time.Millisecond
			}

			if lateQualifiedReqs.Contains(chunkID) {
				// mocks retry after long so they never qualify soon.
				return time.Hour
			}

			return 0

		}, // request info existence
		func(chunkID flow.Identifier) bool {
			if instantQualifiedReqs.Contains(chunkID) || lateQualifiedReqs.Contains(chunkID) {
				return true
			}

			return false
		},
	)

	pendingRequests.On("UpdateRequestHistory", testifymock.Anything, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			// type assertion of inputs.
			chunkID, ok := args[0].(flow.Identifier)
			require.True(t, ok)

			_, ok = args[1].(flowmempool.ChunkRequestHistoryUpdaterFunc)
			require.True(t, ok)

			// checks only instantly qualified chunk requests should reach to this step,
			// i.e., invocation of UpdateRequestHistory
			require.Contains(t, instantQualifiedReqs, chunkID)
			require.NotContains(t, lateQualifiedReqs, chunkID)
			require.NotContains(t, disQualifiedReqs, chunkID)

		}). // makes chunk request instantly qualified for retry, i.e., can be
		// retried anytime after on.
		Return(uint64(1), time.Now(), 1*time.Millisecond, true).
		Times(attempts * len(instantQualifiedReqs))

	return wg
}
