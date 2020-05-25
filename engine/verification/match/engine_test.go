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

func CreateNParticipantsWithMyRole(n int, role flow.Role) (
	flow.IdentityList, flow.Identifier, *module.Local) {
	// initialize the paramaters
	participants := unittest.IdentityListFixture(n, unittest.WithAllRoles())
	myID := participants.Filter(filter.HasRole(role))[0].ID()
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
	participants, myID, me := CreateNParticipantsWithMyRole(5, flow.RoleVerification)
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

	e, err := New(log, ms.net, ms.me, ms.results, ms.verifier, ms.assigner, ms.state, ms.chunks, ms.headers, retryInterval)
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
		}
		assignment.Add(chunk, flow.IdentifierList{assignee})
		return chunk
	}
}

func FromChunk(chunk *flow.Chunk) flow.ChunkDataPack {
	return flow.ChunkDataPack{
		ChunkID: chunk.ID(),
	}
}

func OnChunkDataRequest(con *network.Conduit, ret func(*messages.ChunkDataPackResponse) error) {
	con.On("Submit").Return(ret)
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
	chunkDataPack := FromChunk(myChunk)

	t := ms.T()

	var wg sync.WaitGroup
	wg.Add(1)
	// chunk data was requested once, and return the chunk data pack when requested
	ms.con.On("Submit", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		defer wg.Done()
		req := args.Get(0).(*messages.ChunkDataPackRequest)
		fmt.Printf("con.Submit is called\n")
		// assert the right ID was requested manually as we don't know what nonce was used
		require.Equal(t, myChunk.ID(), req.ChunkID)

		resp := &messages.ChunkDataPackResponse{
			Data:  chunkDataPack,
			Nonce: req.Nonce,
		}

		err := ms.e.Process(en.ID(), resp)
		require.NoError(t, err)

		time.Sleep(1 * time.Second)
	}).Return(nil).Once()

	// check verifier's method is called
	ms.verifier.On("ProcessLocal", mock.Anything).Run(func(args mock.Arguments) {
		vchunk := args.Get(0).(*verification.VerifiableChunkData)
		require.Equal(t, verification.VerifiableChunkData{
			Chunk:         myChunk,
			Header:        ms.head,
			Result:        result,
			ChunkDataPack: &chunkDataPack,
		}, vchunk)

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
}

// // No assignment: When receives a ER, and no chunk is assigned to me, then I won’t fetch any collection or chunk,
// // nor produce any verifiable chunk
// func (ms *MatchSuite) TestNoAssignment(t *testing.T) {
// 	result := WithChunks(1)
//
// 	chunkIndexAssignTo(ms.assigner)(0, ms.otherID)
//
// 	en, err := ms.identities.Filter(ExecutionNode(1)).ID()
// 	require.NoError(t, err)
//
// 	err = ms.e.Submit(en, result)
// 	require.NoError(t, err)
//
// 	chunkDataPack := ForChunk(1)
// 	err = ms.e.Process(en, chunkDataPack)
// 	require.NoError(ms.T(), err)
//
// 	ms.verifier.On("Submit", mock.Anything).Return(nil).Once()
// }
//
// // Multiple Assignments: When receives a ER, and 2 chunks out of 3 are assigned to me,
// // it will produce 2 verifiable chunks.
// func (ms *MatchSuite) TestNoAssignment(t *testing.T) {
// 	result, assignments := createExecutionResult(
// 		WithChunks(
// 			WithAssignee(ms.myID),
// 			WithAssignee(ms.myID),
// 			WithAssignee(ms.otherID),
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
// 	chunkDataPack := ForChunk(1)
// 	err = ms.e.Process(en, chunkDataPack)
// 	require.NoError(ms.T(), err)
//
// 	ms.verifier.On("Submit", mock.Anything).Return(nil).Twice()
// }
//
// // Duplication: When receives 2 ER for the same block, which only has 1 chunk, only 1 verifiable chunk will be produced.
// func (ms *MatchSuite) TestNoAssignment(t *testing.T) {
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
// 	chunkDataPack := ForChunk(1)
// 	err = ms.e.Process(en, chunkDataPack)
// 	require.NoError(ms.T(), err)
//
// 	ms.verifier.On("Submit", mock.Anything).Return(nil).Twice()
// }
//
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
//
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
