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

// when maxAttempt is set to 3, CanTry will only return true for the first 3 times.
func TestCanTry(t *testing.T) {
	t.Run("maxAttempt=3", func(t *testing.T) {
		maxAttempt := 3
		chunks := NewChunks(10)
		chunk := NewChunkStatus(ChunkWithIndex(0), flow.Identifier{0xaa}, flow.Identifier{0xbb})
		chunks.Add(chunk)
		results := []bool{}
		for i := 0; i < 5; i++ {
			results = append(results, CanTry(maxAttempt, chunk))
			chunks.IncrementAttempt(chunk.ID())
		}
		require.Equal(t, []bool{true, true, true, false, false}, results)
	})
}

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
	// set up local module mock
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
	retryInterval := time.Second

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

func WithChunks(setAssignees ...func(uint64, *chunks.Assignment) *flow.Chunk) func(*flow.ExecutionResult, *chunks.Assignment) {
	return func(results *flow.ExecutionResult, assignment *chunks.Assignment) {
		for i, setAssignee := range setAssignees {
			chunk := setAssignee(uint64(i), assignment)
			results.ExecutionResultBody.Chunks.Insert(chunk)
		}
	}
}

func ChunkWithIndex(index int) *flow.Chunk {
	chunk := &flow.Chunk{
		Index: uint64(index),
		ChunkBody: flow.ChunkBody{
			CollectionIndex: uint(index),
		},
	}
	return chunk
}

func WithAssignee(assignee flow.Identifier) func(uint64, *chunks.Assignment) *flow.Chunk {
	return func(index uint64, assignment *chunks.Assignment) *flow.Chunk {
		chunk := ChunkWithIndex(int(index))
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
	assigner.On("Assign", mock.Anything, mock.Anything, mock.Anything).Return(assignment, nil).Once()

	// block header has been received
	headerDB[result.BlockID] = head

	// find the execution node id that created the execution result
	en := participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// create chunk data pack
	myChunk := result.ExecutionResultBody.Chunks[0]
	chunkDataPack := FromChunkID(myChunk.ID())

	var wg sync.WaitGroup
	wg.Add(1)
	// chunk data was requested once, and return the chunk data pack when requested
	// called with 3 mock.Anything, the first is the request, the second and third are the 2
	// execution nodes
	con.On("Submit", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(0).(*messages.ChunkDataPackRequest)
		fmt.Printf("con.Submit is called\n")
		// assert the right ID was requested manually as we don't know what nonce was used
		require.Equal(t, myChunk.ID(), req.ChunkID)

		resp := &messages.ChunkDataPackResponse{
			Data:  FromChunkID(req.ChunkID),
			Nonce: req.Nonce,
		}

		err := e.Process(en.ID(), resp)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
		wg.Done()
	}).Return(nil).Once()

	// check verifier's method is called
	verifier.On("ProcessLocal", mock.Anything).Run(func(args mock.Arguments) {
		vchunk := args.Get(0).(*verification.VerifiableChunkData)
		require.Equal(t, myChunk, vchunk.Chunk)
		require.Equal(t, head, vchunk.Header)
		require.Equal(t, result, vchunk.Result)
		require.Equal(t, &chunkDataPack, vchunk.ChunkDataPack)
	}).Return(nil).Once()

	<-e.Ready()
	fmt.Printf("match.Engine.Process is called\n")
	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(t, err)

	wg.Wait()

	assigner.AssertExpectations(t)
	con.AssertExpectations(t)
	verifier.AssertExpectations(t)
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

	expectedChunkID := make(map[flow.Identifier]struct{})
	expectedChunkID[result.ExecutionResultBody.Chunks[0].ID()] = struct{}{}
	expectedChunkID[result.ExecutionResultBody.Chunks[2].ID()] = struct{}{}

	var wg sync.WaitGroup
	wg.Add(2)
	// chunk data was requested once, and return the chunk data pack when requested
	con.On("Submit", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(0).(*messages.ChunkDataPackRequest)
		fmt.Printf("con.Submit is called\n")
		_, exists := findChunk(result, req.ChunkID)
		require.True(t, exists)
		chunkDataPack := FromChunkID(req.ChunkID)

		resp := &messages.ChunkDataPackResponse{
			Data:  chunkDataPack,
			Nonce: req.Nonce,
		}

		err := e.Process(en.ID(), resp)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
		delete(expectedChunkID, req.ChunkID)
		wg.Done()
	}).Return(nil).Twice()

	verifier.On("ProcessLocal", mock.Anything).Run(func(args mock.Arguments) {
		vchunk := args.Get(0).(*verification.VerifiableChunkData)
		chunk, exists := findChunk(result, vchunk.Chunk.ID())
		require.True(t, exists)
		require.Equal(t, chunk, vchunk.Chunk)
		require.Equal(t, head, vchunk.Header)
		require.Equal(t, result, vchunk.Result)
		chunkDataPack := FromChunkID(chunk.ID())
		require.Equal(t, &chunkDataPack, vchunk.ChunkDataPack)
	}).Return(nil).Twice()

	<-e.Ready()
	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(t, err)

	wg.Wait()

	require.Equal(t, 0, len(expectedChunkID))
	assigner.AssertExpectations(t)
	con.AssertExpectations(t)
	verifier.AssertExpectations(t)
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

	var wg sync.WaitGroup
	wg.Add(1)
	// chunk data was requested once, and return the chunk data pack when requested
	con.On("Submit", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(0).(*messages.ChunkDataPackRequest)
		fmt.Printf("con.Submit is called\n")
		_, exists := findChunk(result, req.ChunkID)
		require.True(t, exists)
		chunkDataPack := FromChunkID(req.ChunkID)

		resp := &messages.ChunkDataPackResponse{
			Data:  chunkDataPack,
			Nonce: req.Nonce,
		}

		err := e.Process(en.ID(), resp)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
		wg.Done()
	}).Return(nil).Once()

	verifier.On("ProcessLocal", mock.Anything).Return(nil).Once()

	<-e.Ready()
	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(t, err)

	// engine processes the execution result again
	err = e.Process(en.ID(), result)
	require.Contains(t, err.Error(), "execution result has been added")
	wg.Wait()

	assigner.AssertExpectations(t)
	con.AssertExpectations(t)
	verifier.AssertExpectations(t)

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

	// create chunk data pack
	myChunk := result.ExecutionResultBody.Chunks[0]

	var wg sync.WaitGroup
	wg.Add(3)
	// chunk data was requested once, and return the chunk data pack when requested
	// called with 3 mock.Anything, the first is the request, the second and third are the 2
	// execution nodes
	callCount := 0
	con.On("Submit", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(0).(*messages.ChunkDataPackRequest)
		fmt.Printf("con.Submit is called\n")
		// assert the right ID was requested manually as we don't know what nonce was used
		require.Equal(t, myChunk.ID(), req.ChunkID)

		callCount++
		if callCount == 3 {

			resp := &messages.ChunkDataPackResponse{
				Data:  FromChunkID(req.ChunkID),
				Nonce: req.Nonce,
			}

			err := e.Process(en.ID(), resp)
			require.NoError(t, err)

			time.Sleep(time.Millisecond)
		}
		wg.Done()
	}).Return(nil).Times(3) // retry 3 times

	// check verifier's method is called
	verifier.On("ProcessLocal", mock.Anything).Return(nil).Once()

	<-e.Ready()
	fmt.Printf("match.Engine.Process is called\n")
	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(t, err)

	wg.Wait()

	assigner.AssertExpectations(t)
	con.AssertExpectations(t)
	verifier.AssertExpectations(t)
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

	var wg sync.WaitGroup
	wg.Add(3)
	con.On("Submit", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		wg.Done()
	}).Return(nil).Times(3)

	<-e.Ready()
	fmt.Printf("match.Engine.Process is called\n")
	// engine processes the execution result
	err := e.Process(en.ID(), result)
	require.NoError(t, err)

	// wait until 3 retry attampts are done
	wg.Wait()

	assigner.AssertExpectations(t)
	con.AssertExpectations(t)
	e.Done()
}

// Concurrency: When 3 different ER are received concurrently, chunks from both
// results will be processed
func TestProcessExecutionResultConcurrently(t *testing.T) {
	e, participants, myID, _, _, _, con, _, _, headerDB, _, _, _, verifier, _, assigner :=
		SetupTest(t, 1)

	ers := make([]*flow.ExecutionResult, 0)

	for i := 0; i < 3; i++ {
		header := &flow.Header{View: uint64(i)}
		// create a execution result that assigns to me
		result, assignment := createExecutionResult(
			header.ID(),
			WithChunks(
				WithAssignee(myID),
			),
		)
		fmt.Printf("headerid;, resultID %v, %v\n", header.ID(), result.ID())
		ers = append(ers, result)

		// add assignment to assigner
		assigner.On("Assign", mock.Anything, result.Chunks, mock.Anything).Return(assignment, nil).Once()

		// block header has been received
		headerDB[result.BlockID] = header
	}

	// find the execution node id that created the execution result
	en := participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	var wg sync.WaitGroup
	wg.Add(3)
	con.On("Submit", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(0).(*messages.ChunkDataPackRequest)
		fmt.Printf("con.Submit is called for chunk: %v\n", req.ChunkID)

		resp := &messages.ChunkDataPackResponse{
			Data:  FromChunkID(req.ChunkID),
			Nonce: req.Nonce,
		}

		err := e.Process(en.ID(), resp)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
		wg.Done()
	}).Return(nil).Times(3)

	// check verifier's method is called
	verifier.On("ProcessLocal", mock.Anything).Return(nil).Times(3)

	<-e.Ready()
	fmt.Printf("match.Engine.Process is called\n")
	// engine processes the execution result concurrently
	for _, result := range ers {
		go func(result *flow.ExecutionResult) {
			err := e.Process(en.ID(), result)
			require.NoError(t, err)
		}(result)
	}

	wg.Wait()

	assigner.AssertExpectations(t)
	con.AssertExpectations(t)
	verifier.AssertExpectations(t)
	e.Done()
}

// Concurrency: When chunk data pack are sent concurrently, match engine is able to receive
// all of them, and process concurrently.
func TestProcessChunkDataPackConcurrently(t *testing.T) {
}
