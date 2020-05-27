package match

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	module "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/module/results"
	netint "github.com/dapperlabs/flow-go/network"
	network "github.com/dapperlabs/flow-go/network/mock"
	protint "github.com/dapperlabs/flow-go/state/protocol"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	storerr "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func FinalizedProtocolStateWithParticipants(participants flow.IdentityList) (
	*flow.Block, *protocol.Snapshot, *protocol.State) {
	block := unittest.BlockFixture()
	head := block.Header

	// set up protocol snapshot mock
	snapshot := &protocol.Snapshot{}
	snapshot.On("Identities", mock.Anything).Return(
		func(filter flow.IdentityFilter) flow.IdentityList {
			return participants.Filter(filter)
		},
		nil,
	)
	snapshot.On("Head").Return(
		func() *flow.Header {
			return head
		},
		nil,
	)

	// set up protocol state mock
	state := &protocol.State{}
	state.On("Final").Return(
		func() protint.Snapshot {
			return snapshot
		},
	)
	state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) protint.Snapshot {
			return snapshot
		},
	)
	return &block, snapshot, state
}

func RegisterNetwork() (*module.Network, *network.Conduit) {
	con := &network.Conduit{}

	// set up network module mock
	net := &module.Network{}
	net.On("Register", mock.Anything, mock.Anything).Return(
		func(code uint8, engine netint.Engine) netint.Conduit {
			return con
		},
		nil,
	)

	return net, con
}

func HeadersFromMap(headerDB map[flow.Identifier]*flow.Header) *storage.Headers {
	headers := &storage.Headers{}
	headers.On("Store", mock.Anything).Return(
		func(header *flow.Header) error {
			headerDB[header.ID()] = header
			return nil
		},
	)
	headers.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Header {
			return headerDB[blockID]
		},
		func(blockID flow.Identifier) error {
			_, exists := headerDB[blockID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	return headers
}

func CreateNParticipantsWithMyRole(myRole flow.Role, otherRoles ...flow.Role) (
	flow.IdentityList, flow.Identifier, *module.Local) {
	// initialize the paramaters
	// participants := unittest.IdentityFixture(myRole)
	participants := make(flow.IdentityList, 0)
	myIdentity := unittest.IdentityFixture(unittest.WithRole(myRole))
	myID := myIdentity.ID()
	participants = append(participants, myIdentity)
	for _, role := range otherRoles {
		id := unittest.IdentityFixture(unittest.WithRole(role))
		participants = append(participants, id)
	}

	// set up local module mock
	me := &module.Local{}
	me.On("NodeID").Return(
		func() flow.Identifier {
			return myID
		},
	)
	return participants, myID, me
}

func SetupTest(t *testing.T, maxTry int) (
	e *Engine,
	participants flow.IdentityList,
	myID flow.Identifier,
	otherID flow.Identifier,
	head *flow.Header,
	me *module.Local,
	con *network.Conduit,
	net *module.Network,
	headers *storage.Headers,
	headerDB map[flow.Identifier]*flow.Header,
	state *protocol.State,
	snapshot *protocol.Snapshot,
	er *results.PendingResults,
	verifier *network.Engine,
	chunks *Chunks,
	assigner *module.ChunkAssigner,
) {
	// setup the network with 2 verification nodes, 3 execution nodes
	// 2 verification nodes are needed to verify chunks, which are assigned to other verification nodes, are ignored.
	// 3 execution nodes are needed to verify chunk data pack requests are sent to only 2 execution nodes
	participants, myID, me = CreateNParticipantsWithMyRole(flow.RoleVerification,
		flow.RoleVerification,
		flow.RoleCollection,
		flow.RoleConsensus,
		flow.RoleExecution,
		flow.RoleExecution,
		flow.RoleExecution,
	)

	// set up network conduit mock
	net, con = RegisterNetwork()

	// set up header storage mock
	headerDB = make(map[flow.Identifier]*flow.Header)
	headers = HeadersFromMap(headerDB)

	// setup protocol state
	block, snapshot, state := FinalizedProtocolStateWithParticipants(participants)
	head = block.Header

	// setup other dependencies
	er = results.NewPendingResults()
	verifier = &network.Engine{}
	assigner = &module.ChunkAssigner{}
	chunks = NewChunks(10)

	log := zerolog.New(os.Stderr)
	retryInterval := 20 * time.Millisecond

	e, err := New(log, net, me, er, verifier, assigner, state, chunks, headers, retryInterval, maxTry)
	require.NoError(t, err)
	return e, participants, myID, otherID, head, me, con, net, headers, headerDB, state, snapshot, er, verifier, chunks, assigner
}

func createExecutionResult(blockID flow.Identifier, options ...func(result *flow.ExecutionResult, assignments *chunks.Assignment)) (*flow.ExecutionResult, *chunks.Assignment) {
	result := &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			BlockID: blockID,
			Chunks:  flow.ChunkList{},
		},
	}
	assignments := chunks.NewAssignment()

	for _, option := range options {
		option(result, assignments)
	}
	return result, assignments
}

func WithChunks(setAssignees ...func(flow.Identifier, uint64, *chunks.Assignment) *flow.Chunk) func(*flow.ExecutionResult, *chunks.Assignment) {
	return func(result *flow.ExecutionResult, assignment *chunks.Assignment) {
		for i, setAssignee := range setAssignees {
			chunk := setAssignee(result.BlockID, uint64(i), assignment)
			result.ExecutionResultBody.Chunks.Insert(chunk)
		}
	}
}

func ChunkWithIndex(blockID flow.Identifier, index int) *flow.Chunk {
	chunk := &flow.Chunk{
		Index: uint64(index),
		ChunkBody: flow.ChunkBody{
			CollectionIndex: uint(index),
			EventCollection: blockID, // ensure chunks from different blocks with the same index will have different chunk ID
		},
	}
	return chunk
}

func WithAssignee(assignee flow.Identifier) func(flow.Identifier, uint64, *chunks.Assignment) *flow.Chunk {
	return func(blockID flow.Identifier, index uint64, assignment *chunks.Assignment) *flow.Chunk {
		chunk := ChunkWithIndex(blockID, int(index))
		fmt.Printf("with assignee: %v, chunk id: %v\n", index, chunk.ID())
		assignment.Add(chunk, flow.IdentifierList{assignee})
		return chunk
	}
}

func FromChunkID(chunkID flow.Identifier) flow.ChunkDataPack {
	return flow.ChunkDataPack{
		ChunkID: chunkID,
	}
}

func ChunkDataPackIsRequestedNTimes(con *network.Conduit, n int, f func(*messages.ChunkDataRequest)) <-chan []*messages.ChunkDataRequest {
	reqs := make([]*messages.ChunkDataRequest, 0)
	c := make(chan []*messages.ChunkDataRequest, 1)

	var wg sync.WaitGroup
	wg.Add(n)
	// chunk data was requested once, and return the chunk data pack when requested
	// called with 3 mock.Anything, the first is the request, the second and third are the 2
	// execution nodes
	con.On("Submit", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(0).(*messages.ChunkDataRequest)
		reqs = append(reqs, req)

		fmt.Printf("con.Submit is called\n")

		go func() {
			f(req)
			wg.Done()
		}()

	}).Return(nil).Times(n)

	go func() {
		wg.Wait()
		c <- reqs
		close(c)
	}()

	return c
}

func RespondChunkDataPack(t *testing.T, engine *Engine, en flow.Identifier) func(*messages.ChunkDataRequest) {
	return func(req *messages.ChunkDataRequest) {
		resp := &messages.ChunkDataResponse{
			ChunkDataPack: FromChunkID(req.ChunkID),
			Nonce:         req.Nonce,
		}

		err := engine.Process(en, resp)
		require.NoError(t, err)
	}
}

func VerifierCalledNTimes(verifier *network.Engine, n int) <-chan []*verification.VerifiableChunkData {
	var wg sync.WaitGroup
	vchunks := make([]*verification.VerifiableChunkData, 0)
	c := make(chan []*verification.VerifiableChunkData, 1)

	wg.Add(n)
	verifier.On("ProcessLocal", mock.Anything).Run(func(args mock.Arguments) {
		vchunk := args.Get(0).(*verification.VerifiableChunkData)
		vchunks = append(vchunks, vchunk)
		wg.Done()
	}).Return(nil).Times(n)

	go func() {
		wg.Wait()
		c <- vchunks
		close(c)
	}()
	return c
}

// Happy Path: When receives a ER, and 1 chunk is assigned to me,
// it will fetch that collection and chunk data, and produces a verifiable chunk
func TestChunkVerified(t *testing.T) {
	e, participants, myID, _, head, _, con, _, _, headerDB, _, _, _, verifier, _, assigner := SetupTest(t, 1)
	// create a execution result that assigns to me
	result, assignment := createExecutionResult(
		head.ID(),
		WithChunks(
			WithAssignee(myID),
		),
	)

	// add assignment to assigner
	assigner.On("Assign", mock.Anything, result.Chunks, mock.Anything).Return(assignment, nil).Once()

	// block header has been received
	headerDB[result.BlockID] = head

	// find the execution node id that created the execution result
	en := participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// create chunk data pack
	myChunk := result.ExecutionResultBody.Chunks[0]
	chunkDataPack := FromChunkID(myChunk.ID())

	// setup conduit to return requested chunk data packs
	// return received requests
	reqsC := ChunkDataPackIsRequestedNTimes(con, 1, RespondChunkDataPack(t, e, en.ID()))

	// check verifier's method is called
	vchunksC := VerifierCalledNTimes(verifier, 1)

	<-e.Ready()

	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(t, err)

	// wait until verifier has been called
	vchunks := <-vchunksC
	reqs := <-reqsC

	require.Equal(t, 1, len(reqs))
	require.Equal(t, myChunk.ID(), reqs[0].ChunkID)

	require.Equal(t, 1, len(vchunks))
	require.Equal(t, head, vchunks[0].Header)
	require.Equal(t, result, vchunks[0].Result)
	require.Equal(t, &chunkDataPack, vchunks[0].ChunkDataPack)

	mock.AssertExpectationsForObjects(t, assigner, con, verifier)
	e.Done()
}

// No assignment: When receives a ER, and no chunk is assigned to me, then I won’t fetch any collection or chunk,
// nor produce any verifiable chunk
func TestNoAssignment(t *testing.T) {
	e, participants, _, otherID, head, _, _, _, _, headerDB, _, _, _, _, _, assigner := SetupTest(t, 1)
	// create a execution result that assigns to me
	result, assignment := createExecutionResult(
		head.ID(),
		WithChunks(
			WithAssignee(otherID),
		),
	)

	// add assignment to assigner
	assigner.On("Assign", mock.Anything, result.Chunks, mock.Anything).Return(assignment, nil).Once()

	// block header has been received
	headerDB[result.BlockID] = head

	// find the execution node id that created the execution result
	en := participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	<-e.Ready()

	err := e.Process(en.ID(), result)
	require.NoError(t, err)
	e.Done()
}

func findChunk(result *flow.ExecutionResult, chunkID flow.Identifier) (*flow.Chunk, bool) {
	for _, chunk := range result.ExecutionResultBody.Chunks {
		if chunk.ID() == chunkID {
			return chunk, true
		}
	}
	return nil, false
}

// Multiple Assignments: When receives a ER, and 2 chunks out of 3 are assigned to me,
// it will produce 2 verifiable chunks.
func TestMultiAssignment(t *testing.T) {
	e, participants, myID, otherID, head, _, con, _, _, headerDB, _, _, _, verifier, _, assigner := SetupTest(t, 1)
	// create a execution result that assigns to me
	result, assignment := createExecutionResult(
		head.ID(),
		WithChunks(
			WithAssignee(myID),
			WithAssignee(otherID),
			WithAssignee(myID),
		),
	)

	// add assignment to assigner
	assigner.On("Assign", mock.Anything, result.Chunks, mock.Anything).Return(assignment, nil).Once()

	// block header has been received
	headerDB[result.BlockID] = head

	// find the execution node id that created the execution result
	en := participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// setup conduit to return requested chunk data packs
	// return received requests
	_ = ChunkDataPackIsRequestedNTimes(con, 2, RespondChunkDataPack(t, e, en.ID()))

	// check verifier's method is called
	vchunksC := VerifierCalledNTimes(verifier, 2)

	<-e.Ready()

	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(t, err)

	// wait until verifier has been called
	vchunks := <-vchunksC

	require.Equal(t, 2, len(vchunks))

	mock.AssertExpectationsForObjects(t, assigner, con, verifier)
	e.Done()
}

// Duplication: When receives 2 ER for the same block, which only has 1 chunk, only 1 verifiable chunk will be produced.
func TestDuplication(t *testing.T) {
	e, participants, myID, otherID, head, _, con, _, _, headerDB, _, _, _, verifier, _, assigner := SetupTest(t, 1)
	// create a execution result that assigns to me
	result, assignment := createExecutionResult(
		head.ID(),
		WithChunks(
			WithAssignee(myID),
			WithAssignee(otherID),
		),
	)

	// add assignment to assigner
	assigner.On("Assign", mock.Anything, result.Chunks, mock.Anything).Return(assignment, nil).Once()

	// block header has been received
	headerDB[result.BlockID] = head

	// find the execution node id that created the execution result
	en := participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// setup conduit to return requested chunk data packs
	// return received requests
	_ = ChunkDataPackIsRequestedNTimes(con, 1, RespondChunkDataPack(t, e, en.ID()))

	// check verifier's method is called
	vchunksC := VerifierCalledNTimes(verifier, 1)

	<-e.Ready()

	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(t, err)

	// engine processes the execution result again
	err = e.Process(en.ID(), result)
	require.Contains(t, err.Error(), "execution result has been added")

	// wait until verifier has been called
	_ = <-vchunksC

	mock.AssertExpectationsForObjects(t, assigner, con, verifier)
	e.Done()
}

// Retry: When receives 1 ER, and 1 chunk is assigned assigned to me, if max retry is 3,
// the execution node fails to return data for the first 2 requests,
// and successful to return in the 3rd try, a verifiable chunk will be produced
func TestRetry(t *testing.T) {
	maxTry := 3
	e, participants, myID, _, head, _, con, _, _, headerDB, _, _, _, verifier, _, assigner := SetupTest(t, maxTry)
	// create a execution result that assigns to me
	result, assignment := createExecutionResult(
		head.ID(),
		WithChunks(
			WithAssignee(myID),
		),
	)

	// add assignment to assigner
	assigner.On("Assign", mock.Anything, result.Chunks, mock.Anything).Return(assignment, nil).Once()

	// block header has been received
	headerDB[result.BlockID] = head

	// find the execution node id that created the execution result
	en := participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// setup conduit to return requested chunk data packs
	// return received requests
	called := 0
	_ = ChunkDataPackIsRequestedNTimes(con, 3, func(req *messages.ChunkDataRequest) {
		called++
		if called >= 3 {
			RespondChunkDataPack(t, e, en.ID())(req)
		}
	})

	// check verifier's method is called
	vchunksC := VerifierCalledNTimes(verifier, 1)

	<-e.Ready()

	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(t, err)

	// engine processes the execution result again
	err = e.Process(en.ID(), result)
	require.Contains(t, err.Error(), "execution result has been added")

	// wait until verifier has been called
	_ = <-vchunksC

	mock.AssertExpectationsForObjects(t, assigner, con, verifier)
	e.Done()
}

// MaxRetry: When receives 1 ER, and 1 chunk is assigned assigned to me, if max retry is 2,
// and the execution node fails to return data for the first 2 requests, then no verifiable chunk will be produced
func TestMaxRetry(t *testing.T) {
	maxAttempt := 3
	e, participants, myID, _, head, _, con, _, _, headerDB, _, _, _, _, _, assigner := SetupTest(t, maxAttempt)
	// create a execution result that assigns to me
	result, assignment := createExecutionResult(
		head.ID(),
		WithChunks(
			WithAssignee(myID),
		),
	)

	// add assignment to assigner
	assigner.On("Assign", mock.Anything, result.Chunks, mock.Anything).Return(assignment, nil).Once()

	// block header has been received
	headerDB[result.BlockID] = head

	// find the execution node id that created the execution result
	en := participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// never returned any chunk data pack
	reqsC := ChunkDataPackIsRequestedNTimes(con, 3, func(req *messages.ChunkDataRequest) {})

	<-e.Ready()

	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(t, err)

	// wait until 3 retry attampts are done
	_ = <-reqsC

	mock.AssertExpectationsForObjects(t, assigner, con)
	e.Done()
}

// Concurrency: When 10 different ER are received concurrently, chunks from both
// results will be processed
func TestProcessExecutionResultConcurrently(t *testing.T) {
	e, participants, myID, _, _, _, con, _, _, headerDB, _, _, _, verifier, _, assigner :=
		SetupTest(t, 1)

	ers := make([]*flow.ExecutionResult, 0)

	count := 10
	for i := 0; i < count; i++ {
		header := &flow.Header{View: uint64(i)}
		// create a execution result that assigns to me
		result, assignment := createExecutionResult(
			header.ID(),
			WithChunks(
				WithAssignee(myID),
			),
		)

		// add assignment to assigner
		assigner.On("Assign", mock.Anything, result.Chunks, mock.Anything).Return(assignment, nil).Once()

		// block header has been received
		headerDB[result.BlockID] = header
	}

	// find the execution node id that created the execution result
	en := participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	_ = ChunkDataPackIsRequestedNTimes(con, count, RespondChunkDataPack(t, e, en.ID()))

	// check verifier's method is called
	vchunksC := VerifierCalledNTimes(verifier, count)

	<-e.Ready()

	fmt.Printf("calling match.Engine.Process\n")
	// engine processes the execution result concurrently
	for _, result := range ers {
		go func(result *flow.ExecutionResult) {
			err := e.Process(en.ID(), result)
			require.NoError(t, err)
		}(result)
	}

	// wait until verifier has been called
	_ = <-vchunksC

	mock.AssertExpectationsForObjects(t, assigner, con, verifier)
	e.Done()
}

// Concurrency: When chunk data pack are sent concurrently, match engine is able to receive
// all of them, and process concurrently.
func TestProcessChunkDataPackConcurrently(t *testing.T) {
	e, participants, myID, _, head, _, con, _, _, headerDB, _, _, _, verifier, _, assigner :=
		SetupTest(t, 1)

	// create a execution result that assigns to me
	result, assignment := createExecutionResult(
		head.ID(),
		WithChunks(
			WithAssignee(myID),
			WithAssignee(myID),
			WithAssignee(myID),
			WithAssignee(myID),
			WithAssignee(myID),
			WithAssignee(myID),
		),
	)

	// add assignment to assigner
	assigner.On("Assign", mock.Anything, result.Chunks, mock.Anything).Return(assignment, nil).Once()

	// block header has been received
	headerDB[result.BlockID] = head

	// find the execution node id that created the execution result
	en := participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	count := 6
	reqsC := ChunkDataPackIsRequestedNTimes(con, count, func(*messages.ChunkDataRequest) {})

	// check verifier's method is called
	vchunksC := VerifierCalledNTimes(verifier, count)

	<-e.Ready()

	fmt.Printf("calling match.Engine.Process\n")

	// engine processes the execution result concurrently
	err := e.Process(en.ID(), result)
	require.NoError(t, err)

	_ = <-vchunksC
	reqs := <-reqsC

	var wg sync.WaitGroup
	for _, req := range reqs {
		wg.Add(1)
		go func(req *messages.ChunkDataRequest) {
			RespondChunkDataPack(t, e, en.ID())(req)
			wg.Done()
		}(req)
	}
	wg.Wait()

	mock.AssertExpectationsForObjects(t, assigner, con, verifier)
	e.Done()
}
