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
	"github.com/stretchr/testify/suite"

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

func TestMatchEngine(t *testing.T) {
	suite.Run(t, new(MatchSuite))
}

type MatchSuite struct {
	suite.Suite

	participants flow.IdentityList
	myID         flow.Identifier
	otherID      flow.Identifier
	head         *flow.Header

	me       *module.Local
	con      *network.Conduit
	net      *module.Network
	headers  *storage.Headers
	headerDB map[flow.Identifier]*flow.Header
	state    *protocol.State
	snapshot *protocol.Snapshot
	results  *results.PendingResults
	verifier *network.Engine
	chunks   *Chunks
	assigner *module.ChunkAssigner

	// engine under test
	e *Engine
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

func (ms *MatchSuite) SetupTest() {
	// set up local module mock
	participants, myID, me := CreateNParticipantsWithMyRole(flow.RoleVerification,
		flow.RoleVerification,
		flow.RoleCollection,
		flow.RoleConsensus,
		flow.RoleExecution,
		flow.RoleExecution,
		flow.RoleExecution,
	)
	ms.participants = participants
	ms.myID = myID
	ms.me = me

	// set up network conduit mock
	net, con := RegisterNetwork()
	ms.net = net
	ms.con = con

	// set up header storage mock
	ms.headerDB = make(map[flow.Identifier]*flow.Header)
	ms.headers = HeadersFromMap(ms.headerDB)

	// setup protocol state
	block, snapshot, state := FinalizedProtocolStateWithParticipants(ms.participants)
	ms.head = block.Header
	ms.snapshot = snapshot
	ms.state = state

	// setup other dependencies
	ms.results = results.NewPendingResults()
	ms.verifier = &network.Engine{}
	ms.assigner = &module.ChunkAssigner{}
	ms.chunks = NewChunks(10)

	log := zerolog.New(os.Stderr)
	retryInterval := time.Second
	maxTry := 1

	e, err := New(log, ms.net, ms.me, ms.results, ms.verifier, ms.assigner, ms.state, ms.chunks, ms.headers, retryInterval, maxTry)
	require.NoError(ms.T(), err)

	ms.e = e
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

func WithAssignee(assignee flow.Identifier) func(uint64, *chunks.Assignment) *flow.Chunk {
	return func(index uint64, assignment *chunks.Assignment) *flow.Chunk {
		chunk := &flow.Chunk{
			Index: index,
			ChunkBody: flow.ChunkBody{
				CollectionIndex: uint(index),
			},
		}
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
func (ms *MatchSuite) TestChunkVerified() {
	// create a execution result that assigns to me
	result, assignment := createExecutionResult(
		ms.head.ID(),
		WithChunks(
			WithAssignee(ms.myID),
		),
	)

	// add assignment to assigner
	ms.assigner.On("Assign", mock.Anything, mock.Anything, mock.Anything).Return(assignment, nil).Once()

	// block header has been received
	ms.headerDB[result.BlockID] = ms.head

	// find the exeuction node id that created the execution result
	en := ms.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// create chunk data pack
	myChunk := result.ExecutionResultBody.Chunks[0]
	chunkDataPack := FromChunkID(myChunk.ID())

	t := ms.T()

	var wg sync.WaitGroup
	wg.Add(1)
	// chunk data was requested once, and return the chunk data pack when requested
	// called with 3 mock.Anything, the first is the request, the second and third are the 2
	// execution nodes
	ms.con.On("Submit", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		defer wg.Done()
		req := args.Get(0).(*messages.ChunkDataPackRequest)
		fmt.Printf("con.Submit is called\n")
		// assert the right ID was requested manually as we don't know what nonce was used
		require.Equal(t, myChunk.ID(), req.ChunkID)

		resp := &messages.ChunkDataPackResponse{
			Data:  FromChunkID(req.ChunkID),
			Nonce: req.Nonce,
		}

		err := ms.e.Process(en.ID(), resp)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
	}).Return(nil).Once()

	// check verifier's method is called
	ms.verifier.On("ProcessLocal", mock.Anything).Run(func(args mock.Arguments) {
		vchunk := args.Get(0).(*verification.VerifiableChunkData)
		require.Equal(t, myChunk, vchunk.Chunk)
		require.Equal(t, ms.head, vchunk.Header)
		require.Equal(t, result, vchunk.Result)
		require.Equal(t, &chunkDataPack, vchunk.ChunkDataPack)
	}).Return(nil).Once()

	<-ms.e.Ready()
	fmt.Printf("match.Engine.Process is called\n")
	// engine processes the execution result
	err := ms.e.Process(en.ID(), result)
	require.NoError(t, err)

	wg.Wait()

	ms.assigner.AssertExpectations(t)
	ms.con.AssertExpectations(t)
	ms.verifier.AssertExpectations(t)
	ms.e.Done()
}

// No assignment: When receives a ER, and no chunk is assigned to me, then I won’t fetch any collection or chunk,
// nor produce any verifiable chunk
func (ms *MatchSuite) TestNoAssignment() {
	// create a execution result that assigns to me
	result, assignment := createExecutionResult(
		ms.head.ID(),
		WithChunks(
			WithAssignee(ms.otherID),
		),
	)

	// add assignment to assigner
	ms.assigner.On("Assign", mock.Anything, mock.Anything, mock.Anything).Return(assignment, nil).Once()

	// block header has been received
	ms.headerDB[result.BlockID] = ms.head

	// find the exeuction node id that created the execution result
	en := ms.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	<-ms.e.Ready()

	err := ms.e.Process(en.ID(), result)
	require.NoError(ms.T(), err)
	ms.e.Done()
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
func (ms *MatchSuite) TestMultiAssignment() {
	// create a execution result that assigns to me
	result, assignment := createExecutionResult(
		ms.head.ID(),
		WithChunks(
			WithAssignee(ms.myID),
			WithAssignee(ms.otherID),
			WithAssignee(ms.myID),
		),
	)

	// add assignment to assigner
	ms.assigner.On("Assign", mock.Anything, mock.Anything, mock.Anything).Return(assignment, nil).Once()

	// block header has been received
	ms.headerDB[result.BlockID] = ms.head

	// find the exeuction node id that created the execution result
	en := ms.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// create chunk data pack
	t := ms.T()

	expectedChunkID := make(map[flow.Identifier]struct{})
	expectedChunkID[result.ExecutionResultBody.Chunks[0].ID()] = struct{}{}
	expectedChunkID[result.ExecutionResultBody.Chunks[2].ID()] = struct{}{}

	var wg sync.WaitGroup
	wg.Add(2)
	// chunk data was requested once, and return the chunk data pack when requested
	ms.con.On("Submit", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		defer wg.Done()
		req := args.Get(0).(*messages.ChunkDataPackRequest)
		fmt.Printf("con.Submit is called\n")
		_, exists := findChunk(result, req.ChunkID)
		require.True(t, exists)
		chunkDataPack := FromChunkID(req.ChunkID)

		resp := &messages.ChunkDataPackResponse{
			Data:  chunkDataPack,
			Nonce: req.Nonce,
		}

		err := ms.e.Process(en.ID(), resp)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
		delete(expectedChunkID, req.ChunkID)
	}).Return(nil).Twice()

	ms.verifier.On("ProcessLocal", mock.Anything).Run(func(args mock.Arguments) {
		vchunk := args.Get(0).(*verification.VerifiableChunkData)
		chunk, exists := findChunk(result, vchunk.Chunk.ID())
		require.True(t, exists)
		require.Equal(t, chunk, vchunk.Chunk)
		require.Equal(t, ms.head, vchunk.Header)
		require.Equal(t, result, vchunk.Result)
		chunkDataPack := FromChunkID(chunk.ID())
		require.Equal(t, &chunkDataPack, vchunk.ChunkDataPack)
	}).Return(nil).Twice()

	<-ms.e.Ready()
	// engine processes the execution result
	err := ms.e.Process(en.ID(), result)
	require.NoError(t, err)

	wg.Wait()

	require.Equal(t, 0, len(expectedChunkID))
	ms.assigner.AssertExpectations(t)
	ms.con.AssertExpectations(t)
	ms.verifier.AssertExpectations(t)
	ms.e.Done()
}

// Duplication: When receives 2 ER for the same block, which only has 1 chunk, only 1 verifiable chunk will be produced.
func (ms *MatchSuite) TestDuplication() {
	// create a execution result that assigns to me
	result, assignment := createExecutionResult(
		ms.head.ID(),
		WithChunks(
			WithAssignee(ms.myID),
			WithAssignee(ms.otherID),
		),
	)

	// add assignment to assigner
	ms.assigner.On("Assign", mock.Anything, mock.Anything, mock.Anything).Return(assignment, nil).Once()

	// block header has been received
	ms.headerDB[result.BlockID] = ms.head

	// find the exeuction node id that created the execution result
	en := ms.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// create chunk data pack
	t := ms.T()

	var wg sync.WaitGroup
	wg.Add(1)
	// chunk data was requested once, and return the chunk data pack when requested
	ms.con.On("Submit", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		defer wg.Done()
		req := args.Get(0).(*messages.ChunkDataPackRequest)
		fmt.Printf("con.Submit is called\n")
		_, exists := findChunk(result, req.ChunkID)
		require.True(t, exists)
		chunkDataPack := FromChunkID(req.ChunkID)

		resp := &messages.ChunkDataPackResponse{
			Data:  chunkDataPack,
			Nonce: req.Nonce,
		}

		err := ms.e.Process(en.ID(), resp)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
	}).Return(nil).Once()

	ms.verifier.On("ProcessLocal", mock.Anything).Return(nil).Once()

	<-ms.e.Ready()
	// engine processes the execution result
	err := ms.e.Process(en.ID(), result)
	require.NoError(t, err)

	// engine processes the execution result again
	err = ms.e.Process(en.ID(), result)
	require.Contains(t, err.Error(), "execution result has been added")
	wg.Wait()

	ms.assigner.AssertExpectations(t)
	ms.con.AssertExpectations(t)
	ms.verifier.AssertExpectations(t)

	ms.e.Done()
}

// // Retry: When receives 1 ER, and 1 chunk is assigned assigned to me, if max retry is 3,
// // the execution node fails to return data for the first 2 requests,
// // and successful to return in the 3rd try, a verifiable chunk will be produced
// func (ms *MatchSuite) TestRetry(t *testing.T) {
// 	result, assignments := createExecutionResult(
// 		WithChunks(
// 			WithAssignee(ms.myID),
// 		),
// 	)
//
// 	AddAssignments(ms.assigner, assignments)
//
// 	en, err := findNodeByRoleAndIndex(Role.ExecutionNode, 0)
// 	require.NoError(t, err)
//
// 	err = ms.e.Process(en, result)
// 	require.NoError(t, err)
//
// 	OnChunkDataRequest(ms, func(m *messages.ChunkDataPackRequest) error {
// 		count, exists := ms.request[m.ChunkID]
// 		if !exists || count < 2 {
// 			return nil
// 		}
//
// 		chunkDataPack := ForChunk(1)
// 		err = ms.e.Process(en, chunkDataPack)
// 		require.NoError(ms.T(), err)
// 		return nil
// 	})
//
// 	ms.assertVerifiable(func(t *testing.T, verifiableChunks []*verification.VerifiableChunkData) {
// 		require.Len(t, verifiableChunks, 1)
// 	}, t)
// }

// // MaxRetry: When receives 1 ER, and 1 chunk is assigned assigned to me, if max retry is 2,
// // and the execution node fails to return data for the first 2 requests, then no verifiable chunk will be produced
// func (ms *MatchSuite) TestRetry(t *testing.T) {
// 	// create a execution result that assigns to me
// 	result, assignments := createExecutionResult(
// 		WithChunks(
// 			WithAssignee(ms.myID),
// 		),
// 	)
//
// 	// add assignments to assigner
// 	AddAssignments(ms.assigner, assignments)
//
// 	// find the exeuction node id that created the execution result
// 	en := ms.participants.Filter(filter.HasRole(flow.RoleExecution))[0]
//
// 	// proess the result
// 	err := ms.e.Process(en, result)
// 	require.NoError(t, err)
//
// 	// when the execution node receives chunk data pack request, it will
// 	// reject the first 3 requests
// 	OnChunkDataRequest(ms, func(m *messages.ChunkDataPackRequest) error {
// 		count, exists := ms.request[m.ChunkID]
// 		if !exists || count < 3 {
// 			return nil
// 		}
//
// 		chunkDataPack := ForChunk(1)
// 		err = ms.e.Process(en.ID(), chunkDataPack)
// 		require.NoError(ms.T(), err)
// 		return nil
// 	})
//
// 	// verify that the verifier engine was not called, no verifiable chunk was produced
// 	ms.assertVerifiable(func(t *testing.T, verifiableChunks []*verification.VerifiableChunkData) {
// 		require.Len(t, verifiableChunks, 0)
// 	}, t)
// }
//
// // Concurrency: When 2 different ER are received concurrently, chunks from both
// // results will be processed
//
// // Concurrency: When chunk data pack are sent concurrently, match engine is able to receive
// // all of them, and process concurrently.
