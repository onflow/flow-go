package match_test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/verification"
	"github.com/onflow/flow-go/engine/verification/match"
	"github.com/onflow/flow-go/engine/verification/test"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	realModule "github.com/onflow/flow-go/module"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type MatchEngineTestSuite struct {
	suite.Suite
	net          *module.Network
	me           *module.Local
	participants flow.IdentityList

	metrics *module.VerificationMetrics
	tracer  realModule.Tracer
	myID    flow.Identifier
	head    *flow.Header

	con *mocknetwork.Conduit

	headers          *storage.Headers
	headerDB         map[flow.Identifier]*flow.Header
	state            *protocol.State
	snapshot         *protocol.Snapshot
	sealed           *protocol.Snapshot
	results          *stdmap.ResultDataPacks
	chunkIDsByResult *mempool.IdentifierMap
	verifier         *mocknetwork.Engine
	chunks           *match.Chunks
	assigner         *module.ChunkAssigner
}

func hashResult(res *flow.ExecutionResult) []byte {
	h := hash.NewSHA3_384()

	// encodes result approval body to byte slice
	b, _ := encoding.DefaultEncoder.Encode(res)

	// takes hash of result approval body
	hash := h.ComputeHash(b)

	return hash
}

// TestMatchEngine executes all MatchEngineTestSuite tests.
func TestMatchEngine(t *testing.T) {
	suite.Run(t, new(MatchEngineTestSuite))
}

// SetupTest initiates the test setups prior to each test.
func (suite *MatchEngineTestSuite) SetupTest() {
	// setup the network with 2 verification nodes, 3 execution nodes
	// 2 verification nodes are needed to verify chunks, which are assigned to other verification nodes, are ignored.
	// 3 execution nodes are needed to verify chunk data pack requests are sent to only 2 execution nodes
	participants, myID, me := unittest.CreateNParticipantsWithMyRole(flow.RoleVerification,
		flow.RoleVerification,
		flow.RoleCollection,
		flow.RoleConsensus,
		flow.RoleExecution,
		flow.RoleExecution,
		flow.RoleExecution,
	)

	suite.participants = participants
	suite.myID = myID
	suite.me = me

	// set up network conduit mock
	suite.net, suite.con = unittest.RegisterNetwork()

	// set up header storage mock
	suite.headerDB = make(map[flow.Identifier]*flow.Header)
	suite.headers = unittest.HeadersFromMap(suite.headerDB)

	// setup protocol state
	block, snapshot, state, sealed := unittest.FinalizedProtocolStateWithParticipants(participants)
	suite.head = block.Header
	suite.snapshot = snapshot
	suite.state = state
	suite.sealed = sealed

	// setup other dependencies
	suite.results = stdmap.NewResultDataPacks(10)
	suite.verifier = &mocknetwork.Engine{}
	suite.assigner = &module.ChunkAssigner{}
	suite.metrics = &module.VerificationMetrics{}
	suite.chunkIDsByResult = &mempool.IdentifierMap{}
	suite.tracer = trace.NewNoopTracer()
	suite.chunks = match.NewChunks(10)
}

func (suite *MatchEngineTestSuite) ChunkDataPackIsRequestedNTimes(timeout time.Duration,
	executorID flow.Identifier,
	n int, f func(*messages.ChunkDataRequest)) <-chan []*messages.ChunkDataRequest {
	reqs := make([]*messages.ChunkDataRequest, 0)
	c := make(chan []*messages.ChunkDataRequest, 1)

	wg := &sync.WaitGroup{}

	// to counter race condition in concurrent invocations of Run
	mutex := &sync.Mutex{}
	wg.Add(n)
	// chunk data was requested once, and return the chunk data pack when requested
	// called with 3 mock.Anything, the first is the request, the second and third are the 2
	// execution nodes
	suite.con.On("Publish", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		mutex.Lock()
		defer mutex.Unlock()

		req := args.Get(0).(*messages.ChunkDataRequest)
		reqs = append(reqs, req)

		// chunk data request should be only dispatched to the executor ID
		targetID := args.Get(1).(flow.Identifier)
		require.Equal(suite.T(), targetID, executorID)

		fmt.Printf("con.Submit is called for chunk:%v\n", req.ChunkID)

		go func() {
			f(req)
			wg.Done()
		}()

	}).Return(nil).Times(n)

	go func() {
		unittest.AssertReturnsBefore(suite.T(), wg.Wait, timeout)
		c <- reqs
		close(c)
	}()

	return c
}

func (suite *MatchEngineTestSuite) RespondChunkDataPack(engine *match.Engine,
	en flow.Identifier) func(*messages.ChunkDataRequest) {
	return func(req *messages.ChunkDataRequest) {
		resp := &messages.ChunkDataResponse{
			ChunkDataPack: test.FromChunkID(req.ChunkID),
			Nonce:         req.Nonce,
		}

		err := engine.Process(en, resp)
		require.NoError(suite.T(), err)
	}
}

func (suite *MatchEngineTestSuite) VerifierCalledNTimes(timeout time.Duration, n int) <-chan []*verification.VerifiableChunkData {
	wg := &sync.WaitGroup{}
	vchunks := make([]*verification.VerifiableChunkData, 0)
	c := make(chan []*verification.VerifiableChunkData, 1)

	wg.Add(n)

	// to counter race condition in concurrent invocations of Run
	mutex := &sync.Mutex{}
	suite.verifier.On("ProcessLocal", mock.Anything).Run(func(args mock.Arguments) {
		mutex.Lock()
		defer mutex.Unlock()

		vchunk := args.Get(0).(*verification.VerifiableChunkData)
		vchunks = append(vchunks, vchunk)

		wg.Done()
	}).Return(nil).Times(n)

	go func() {
		unittest.AssertReturnsBefore(suite.T(), wg.Wait, timeout)
		c <- vchunks
		close(c)
	}()
	return c
}

func (suite *MatchEngineTestSuite) OnVerifiableChunkSentMetricCalledNTimes(n int) <-chan struct{} {
	var wg sync.WaitGroup
	c := make(chan struct{}, 1)

	wg.Add(n)
	suite.metrics.On("OnVerifiableChunkSent").Run(func(args mock.Arguments) {
		wg.Done()
	}).Return().Times(n)

	go func() {
		wg.Wait()
		c <- struct{}{}
		close(c)
	}()
	return c
}

// Happy Path: When receives a ER, and 1 chunk is assigned to me,
// it will fetch that collection and chunk data, and produces a verifiable chunk
func (suite *MatchEngineTestSuite) TestChunkVerified() {
	e := suite.NewTestMatchEngine(1)

	// create a execution result that assigns to me
	result, assignment := test.CreateExecutionResult(
		suite.head.ID(),
		test.WithChunks(
			test.WithAssignee(suite.myID),
		),
	)

	seed := hashResult(result)
	suite.snapshot.On("Seed", mock.Anything, mock.Anything, mock.Anything).Return(seed, nil)

	// metrics
	// receiving an execution result
	suite.metrics.On("OnExecutionResultReceived").Return().Once()
	// sending a verifiable chunk
	suite.metrics.On("OnVerifiableChunkSent").Return().Once()
	// receiving a chunk data pack
	suite.metrics.On("OnChunkDataPackReceived").Return().Once()

	// add assignment to assigner
	suite.assigner.On("Assign", result, result.BlockID).Return(assignment, nil).Once()

	// assigned chunk IDs successfully attached to their result ID
	resultID := result.ID()
	for _, chunkIndex := range assignment.ByNodeID(suite.myID) {
		chunkID := result.Chunks[chunkIndex].ID()
		suite.chunkIDsByResult.On("Append", resultID, chunkID).
			Return(nil).Once()
		// mocks resource clean up for assigned chunks
		suite.chunkIDsByResult.On("RemIdFromKey", resultID, chunkID).Return(nil).Once()
	}
	suite.chunkIDsByResult.On("Has", resultID).Return(true)

	// block header has been received
	suite.headerDB[result.BlockID] = suite.head

	// find the execution node id that created the execution result
	en := suite.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// create chunk data pack
	myChunk := result.Chunks[0]
	chunkDataPack := test.FromChunkID(myChunk.ID())

	// setup conduit to return requested chunk data packs
	// return received requests
	reqsC := suite.ChunkDataPackIsRequestedNTimes(5*time.Second, en.NodeID, 1, suite.RespondChunkDataPack(e, en.ID()))

	// check verifier's method is called
	vchunksC := suite.VerifierCalledNTimes(5*time.Second, 1)

	<-e.Ready()

	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(suite.T(), err)

	// wait until verifier has been called
	vchunks := <-vchunksC
	reqs := <-reqsC

	require.Equal(suite.T(), 1, len(reqs))
	require.Equal(suite.T(), myChunk.ID(), reqs[0].ChunkID)

	require.Equal(suite.T(), 1, len(vchunks))
	require.Equal(suite.T(), suite.head, vchunks[0].Header)
	require.Equal(suite.T(), result, vchunks[0].Result)
	require.Equal(suite.T(), &chunkDataPack, vchunks[0].ChunkDataPack)

	<-e.Done()

	mock.AssertExpectationsForObjects(suite.T(),
		suite.assigner,
		suite.con,
		suite.verifier,
		suite.metrics,
		suite.chunkIDsByResult)
}

// No assignment: When receives a ER, and no chunk is assigned to me, then I won’t fetch any collection or chunk,
// nor produce any verifiable chunk
func (suite *MatchEngineTestSuite) TestNoAssignment() {
	e := suite.NewTestMatchEngine(1)

	// create a execution result that assigns to me
	result, assignment := test.CreateExecutionResult(
		suite.head.ID(),
		test.WithChunks(
			test.WithAssignee(flow.Identifier{}),
		),
	)

	seed := hashResult(result)
	suite.snapshot.On("Seed", mock.Anything, mock.Anything, mock.Anything).Return(seed, nil)

	// metrics

	// receiving an execution result
	suite.metrics.On("OnExecutionResultReceived").Return().Once()

	// add MyChunk method to return to assigner
	suite.assigner.On("Assign", result, result.BlockID).Return(assignment, nil).Once()

	// block header has been received
	suite.headerDB[result.BlockID] = suite.head

	// find the execution node id that created the execution result
	en := suite.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	<-e.Ready()

	err := e.Process(en.ID(), result)
	require.NoError(suite.T(), err)

	<-e.Done()

	// result with no chunk assignment should not be stored in mempool
	require.True(suite.T(), suite.results.Size() == 0)
	mock.AssertExpectationsForObjects(suite.T(), suite.metrics)
}

// Multiple Assignments: When receives a ER, and 2 chunks out of 3 are assigned to me,
// it will produce 2 verifiable chunks.
func (suite *MatchEngineTestSuite) TestMultiAssignment() {
	e := suite.NewTestMatchEngine(1)

	// create a execution result that assigns to me
	result, assignment := test.CreateExecutionResult(
		suite.head.ID(),
		test.WithChunks(
			test.WithAssignee(suite.myID),
			test.WithAssignee(flow.Identifier{}), // some other node
			test.WithAssignee(suite.myID),
		),
	)

	seed := hashResult(result)
	suite.snapshot.On("Seed", mock.Anything, mock.Anything, mock.Anything).Return(seed, nil)

	// metrics
	// receiving an execution result
	suite.metrics.On("OnExecutionResultReceived").Return().Once()
	// sending two verifiable chunks
	suite.metrics.On("OnVerifiableChunkSent").Return().Twice()
	// receiving two chunk data packs
	suite.metrics.On("OnChunkDataPackReceived").Return().Twice()

	// add assignment to assigner
	suite.assigner.On("Assign", result, result.BlockID).Return(assignment, nil).Once()

	// assigned chunk IDs successfully attached to their result ID
	resultID := result.ID()
	for _, chunkIndex := range assignment.ByNodeID(suite.myID) {
		chunkID := result.Chunks[chunkIndex].ID()
		suite.chunkIDsByResult.On("Append", resultID, chunkID).
			Return(nil).Once()
		// mocks resource clean up for assigned chunks
		suite.chunkIDsByResult.On("RemIdFromKey", resultID, chunkID).Return(nil).Once()
	}
	suite.chunkIDsByResult.On("Has", resultID).Return(true)

	// block header has been received
	suite.headerDB[result.BlockID] = suite.head

	// find the execution node id that created the execution result
	en := suite.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// setup conduit to return requested chunk data packs
	// return received requests
	_ = suite.ChunkDataPackIsRequestedNTimes(5*time.Second, en.NodeID, 2, suite.RespondChunkDataPack(e, en.ID()))

	// check verifier's method is called
	vchunksC := suite.VerifierCalledNTimes(5*time.Second, 2)

	<-e.Ready()

	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(suite.T(), err)

	// wait until verifier has been called
	vchunks := <-vchunksC

	require.Equal(suite.T(), 2, len(vchunks))

	time.Sleep(1 * time.Second)
	<-e.Done()

	mock.AssertExpectationsForObjects(suite.T(),
		suite.assigner,
		suite.con,
		suite.verifier,
		suite.chunkIDsByResult)
}

// TestDuplication checks that when the engine receives 2 ER for the same block,
// which only has 1 chunk, only 1 verifiable chunk will be produced.
func (suite *MatchEngineTestSuite) TestDuplication() {
	e := suite.NewTestMatchEngine(3)

	// create a execution result that assigns to me
	result, assignment := test.CreateExecutionResult(
		suite.head.ID(),
		test.WithChunks(
			test.WithAssignee(suite.myID),
		),
	)

	seed := hashResult(result)
	suite.snapshot.On("Seed", mock.Anything, mock.Anything, mock.Anything).Return(seed, nil)

	// metrics
	// receiving an execution result
	suite.metrics.On("OnExecutionResultReceived").Return().Twice()
	// sending one verifiable chunks
	suite.metrics.On("OnVerifiableChunkSent").Return().Once()
	// receiving one chunk data packs
	suite.metrics.On("OnChunkDataPackReceived").Return().Once()

	// add assignment to assigner
	suite.assigner.On("Assign", result, result.BlockID).Return(assignment, nil)

	// assigned chunk IDs successfully attached to their result ID
	resultID := result.ID()
	for _, chunkIndex := range assignment.ByNodeID(suite.myID) {
		chunkID := result.Chunks[chunkIndex].ID()
		suite.chunkIDsByResult.On("Append", resultID, chunkID).
			Return(nil).Once()
		// mocks resource clean up for assigned chunks
		suite.chunkIDsByResult.On("RemIdFromKey", resultID, chunkID).Return(nil).Once()
	}
	suite.chunkIDsByResult.On("Has", resultID).Return(true)

	// block header has been received
	suite.headerDB[result.BlockID] = suite.head

	// find the execution node id that created the execution result
	en := suite.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// setup conduit to return requested chunk data packs
	// return received requests
	called := 0
	_ = suite.ChunkDataPackIsRequestedNTimes(5*time.Second, en.NodeID, 3,
		func(req *messages.ChunkDataRequest) {
			called++
			if called >= 3 {
				suite.RespondChunkDataPack(e, en.ID())(req)
			}
		})

	// check verifier's method is called
	vchunkC := suite.VerifierCalledNTimes(5*time.Second, 1)

	<-e.Ready()

	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(suite.T(), err)

	// engine processes the execution result again
	err = e.Process(en.ID(), result)
	require.NoError(suite.T(), err)

	<-vchunkC

	<-e.Done()
	mock.AssertExpectationsForObjects(suite.T(),
		suite.assigner,
		suite.con,
		suite.verifier,
		suite.metrics,
		suite.chunkIDsByResult)
}

// Retry: When receives 1 ER, and 1 chunk is assigned assigned to me, if max retry is 3,
// the execution node fails to return data for the first 2 requests,
// and successful to return in the 3rd try, a verifiable chunk will be produced
func (suite *MatchEngineTestSuite) TestRetry() {
	e := suite.NewTestMatchEngine(3)

	// create a execution result that assigns to me
	result, assignment := test.CreateExecutionResult(
		suite.head.ID(),
		test.WithChunks(
			test.WithAssignee(suite.myID),
		),
	)

	seed := hashResult(result)
	suite.snapshot.On("Seed", mock.Anything, mock.Anything, mock.Anything).Return(seed, nil)

	// metrics
	// receiving an execution result
	suite.metrics.On("OnExecutionResultReceived").Return().Once()
	// sending one verifiable chunk
	suite.metrics.On("OnVerifiableChunkSent").Return().Once()
	// receiving one chunk data pack
	suite.metrics.On("OnChunkDataPackReceived").Return().Once()

	// add assignment to assigner
	suite.assigner.On("Assign", result, result.BlockID).Return(assignment, nil).Once()

	// assigned chunk IDs successfully attached to their result ID
	resultID := result.ID()
	for _, chunkIndex := range assignment.ByNodeID(suite.myID) {
		chunkID := result.Chunks[chunkIndex].ID()
		suite.chunkIDsByResult.On("Append", resultID, chunkID).
			Return(nil).Once()
		// mocks resource clean up for assigned chunks
		suite.chunkIDsByResult.On("RemIdFromKey", resultID, chunkID).Return(nil).Once()
	}
	suite.chunkIDsByResult.On("Has", resultID).Return(true)

	// block header has been received
	suite.headerDB[result.BlockID] = suite.head

	// find the execution node id that created the execution result
	en := suite.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// setup conduit to return requested chunk data packs
	// return received requests
	called := 0
	_ = suite.ChunkDataPackIsRequestedNTimes(5*time.Second, en.NodeID, 3,
		func(req *messages.ChunkDataRequest) {
			called++
			if called >= 3 {
				suite.RespondChunkDataPack(e, en.ID())(req)
			}
		})

	// check verifier's method is called
	vchunkC := suite.VerifierCalledNTimes(5*time.Second, 1)

	<-e.Ready()

	err := e.Process(en.ID(), result)
	require.NoError(suite.T(), err)

	<-vchunkC

	<-e.Done()
	mock.AssertExpectationsForObjects(suite.T(),
		suite.assigner,
		suite.con,
		suite.verifier,
		suite.metrics,
		suite.chunkIDsByResult)
}

// MaxRetry: When receives 1 ER, and 1 chunk is assigned assigned to me, if max retry is 2,
// and the execution node fails to return data for the first 2 requests, then no verifiable chunk will be produced
func (suite *MatchEngineTestSuite) TestMaxRetry() {
	e := suite.NewTestMatchEngine(3)
	// create a execution result that assigns to me
	result, assignment := test.CreateExecutionResult(
		suite.head.ID(),
		test.WithChunks(
			test.WithAssignee(suite.myID),
		),
	)

	seed := hashResult(result)
	suite.snapshot.On("Seed", mock.Anything, mock.Anything, mock.Anything).Return(seed, nil)

	// metrics
	// receiving an execution result
	suite.metrics.On("OnExecutionResultReceived").Return().Once()

	// add assignment to assigner
	suite.assigner.On("Assign", result, result.BlockID).Return(assignment, nil).Once()

	// assigned chunk IDs successfully attached to their result ID
	resultID := result.ID()
	for _, chunkIndex := range assignment.ByNodeID(suite.myID) {
		chunkID := result.Chunks[chunkIndex].ID()
		suite.chunkIDsByResult.On("Append", resultID, chunkID).
			Return(nil).Once()
	}

	// block header has been received
	suite.headerDB[result.BlockID] = suite.head

	// find the execution node id that created the execution result
	en := suite.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// never returned any chunk data pack
	reqC := suite.ChunkDataPackIsRequestedNTimes(5*time.Second, en.NodeID, 3, func(req *messages.ChunkDataRequest) {})

	<-e.Ready()

	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(suite.T(), err)

	<-reqC

	<-e.Done()
	mock.AssertExpectationsForObjects(suite.T(),
		suite.assigner,
		suite.con,
		suite.metrics,
		suite.chunkIDsByResult)
}

// Concurrency: When 10 different ER are received concurrently, chunks from both
// results will be processed
func (suite *MatchEngineTestSuite) TestProcessExecutionResultConcurrently() {
	e := suite.NewTestMatchEngine(1)

	ers := make([]*flow.ExecutionResult, 0)

	count := 10

	// metrics
	// receiving `count`-many result
	suite.metrics.On("OnExecutionResultReceived").Return().Times(count)
	// sending `count`-many verifiable chunks
	suite.metrics.On("OnVerifiableChunkSent").Return().Times(count)
	// receiving `count`-many chunk data packs
	suite.metrics.On("OnChunkDataPackReceived").Return().Times(count)

	for i := 0; i < count; i++ {
		header := &flow.Header{
			Height: suite.head.Height + 1, // ensure the height is above the sealed height
			View:   uint64(i),
		}
		// create a execution result that assigns to me
		result, assignment := test.CreateExecutionResult(
			header.ID(),
			test.WithChunks(
				test.WithAssignee(suite.myID),
			),
		)

		seed := hashResult(result)
		suite.snapshot.On("Seed", mock.Anything, mock.Anything, mock.Anything).Return(seed, nil)

		// add assignment to assigner
		suite.assigner.On("Assign", result, result.BlockID).Return(assignment, nil).Once()

		// assigned chunk IDs successfully attached to their result ID
		resultID := result.ID()
		for _, chunkIndex := range assignment.ByNodeID(suite.myID) {
			chunkID := result.Chunks[chunkIndex].ID()
			suite.chunkIDsByResult.On("Append", resultID, chunkID).
				Return(nil).Once()
			// mocks resource clean up for assigned chunks
			suite.chunkIDsByResult.On("RemIdFromKey", resultID, chunkID).Return(nil).Once()
		}
		suite.chunkIDsByResult.On("Has", resultID).Return(true)

		// block header has been received
		suite.headerDB[result.BlockID] = header
		ers = append(ers, result)
	}

	// find the execution node id that created the execution result
	en := suite.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	_ = suite.ChunkDataPackIsRequestedNTimes(5*time.Second, en.NodeID, count, suite.RespondChunkDataPack(e, en.ID()))

	// check verifier's method is called
	vchunkC := suite.VerifierCalledNTimes(5*time.Second, count)

	<-e.Ready()

	// engine processes the execution result concurrently
	for _, result := range ers {
		go func(result *flow.ExecutionResult) {
			err := e.Process(en.ID(), result)
			require.NoError(suite.T(), err)
		}(result)
	}

	// wait until verifier has been called
	<-vchunkC

	<-e.Done()
	mock.AssertExpectationsForObjects(suite.T(),
		suite.assigner,
		suite.con,
		suite.verifier,
		suite.metrics,
		suite.chunkIDsByResult)
}

// Concurrency: When chunk data pack are sent concurrently, match engine is able to receive
// all of them, and process concurrently.
func (suite *MatchEngineTestSuite) TestProcessChunkDataPackConcurrently() {
	e := suite.NewTestMatchEngine(1)

	// create a execution result that assigns to me
	result, assignment := test.CreateExecutionResult(
		suite.head.ID(),
		test.WithChunks(
			test.WithAssignee(suite.myID),
			test.WithAssignee(suite.myID),
			test.WithAssignee(suite.myID),
			test.WithAssignee(suite.myID),
			test.WithAssignee(suite.myID),
			test.WithAssignee(suite.myID),
		),
	)

	seed := hashResult(result)
	suite.snapshot.On("Seed", mock.Anything, mock.Anything, mock.Anything).Return(seed, nil)

	// metrics
	// receiving `len(result.Chunk)`-many result
	suite.metrics.On("OnExecutionResultReceived").Return().Once()
	// sending `len(result.Chunk)`-many verifiable chunks
	sentMetricsC := suite.OnVerifiableChunkSentMetricCalledNTimes(len(result.Chunks))
	// receiving `len(result.Chunk)`-many chunk data packs
	suite.metrics.On("OnChunkDataPackReceived").Return().Times(len(result.Chunks))

	// add assignment to assigner
	suite.assigner.On("Assign", result, result.BlockID).Return(assignment, nil).Once()

	// assigned chunk IDs successfully attached to their result ID
	resultID := result.ID()
	for _, chunkIndex := range assignment.ByNodeID(suite.myID) {
		chunkID := result.Chunks[chunkIndex].ID()
		suite.chunkIDsByResult.On("Append", resultID, chunkID).
			Return(nil).Once()
		// mocks resource clean up for assigned chunks
		suite.chunkIDsByResult.On("RemIdFromKey", resultID, chunkID).Return(nil).Once()
	}
	suite.chunkIDsByResult.On("Has", resultID).Return(true)

	// block header has been received
	suite.headerDB[result.BlockID] = suite.head

	// find the execution node id that created the execution result
	en := suite.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	count := len(result.Chunks)
	reqsC := suite.ChunkDataPackIsRequestedNTimes(5*time.Second, en.NodeID, count, func(*messages.ChunkDataRequest) {})

	// check verifier's method is called
	_ = suite.VerifierCalledNTimes(5*time.Second, count)

	<-e.Ready()

	// engine processes the execution result concurrently
	err := e.Process(en.ID(), result)
	require.NoError(suite.T(), err)

	reqs := <-reqsC

	// send chunk data pack responses concurrently
	wg := sync.WaitGroup{}
	for _, req := range reqs {
		wg.Add(1)
		go func(req *messages.ChunkDataRequest) {
			suite.RespondChunkDataPack(e, en.ID())(req)
			wg.Done()
		}(req)
	}
	wg.Wait()

	// wait until verifier metrics are called
	// this indicates end of matching all assigned chunks
	unittest.AssertClosesBefore(suite.T(), sentMetricsC, 1*time.Second)

	<-e.Done()
	mock.AssertExpectationsForObjects(suite.T(),
		suite.assigner,
		suite.con,
		suite.verifier,
		suite.metrics,
		suite.chunkIDsByResult)
}

// NewTestMatchEngine tests the establishment of the network registration upon
// creation of an instance of Match Engine using the New method.
// It also returns an instance of new engine to be used in the later tests.
func (suite *MatchEngineTestSuite) NewTestMatchEngine(maxTry int) *match.Engine {
	e, err := match.New(zerolog.New(os.Stderr),
		suite.metrics,
		suite.tracer,
		suite.net,
		suite.me,
		suite.results,
		suite.chunkIDsByResult,
		suite.verifier,
		suite.assigner,
		suite.state,
		suite.chunks,
		suite.headers,
		100*time.Millisecond,
		maxTry)
	require.Nil(suite.T(), err)

	return e
}
