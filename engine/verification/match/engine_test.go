package match

import (
	"os"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	module "github.com/dapperlabs/flow-go/module/mock"
	netint "github.com/dapperlabs/flow-go/network"
	network "github.com/dapperlabs/flow-go/network/mock"
	protint "github.com/dapperlabs/flow-go/state/protocol"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	storerr "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestMatchEngine(t *testing.T) {
	suite.Run(t, new(MatchSuite))
}

type MatchSuite struct {
	suite.Suite

	participants flow.IdentityList
	myID         flow.Identifier
	head         *flow.Header

	me       *module.Local
	con      *network.Conduit
	net      *module.Network
	headers  *storage.Headers
	headerDB map[flow.Identifier]*flow.Header
	state    *protocol.State
	snapshot *protocol.Snapshot
	mutator  *protocol.Mutator
	assigner module.ChunkAssigner // used to determine chunks this node needs to verify

	// engine under test
	e *Engine
}

func (ms *MatchSuite) SetupTest() {
	// initialize the paramaters
	ms.participants = unittest.IdentityListFixture(5, unittest.WithAllRoles())
	ms.myID = ms.participants.Filter(filter.HasRole(flow.RoleVerification))[0]

	// set up network conduit mock
	ms.con = &network.Conduit{}
	ms.con.On("Submit", mock.Anything, mock.Anything).Return(nil)
	ms.con.On("Submit", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ms.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// set up network module mock
	ms.net = &module.Network{}
	ms.net.On("Register", mock.Anything, mock.Anything).Return(
		func(code uint8, engine netint.Engine) netint.Conduit {
			return ms.con
		},
		nil,
	)

	// set up header storage mock
	ms.headers = &storage.Headers{}
	ms.headers.On("Store", mock.Anything).Return(
		func(header *flow.Header) error {
			ms.headerDB[header.ID()] = header
			return nil
		},
	)
	ms.headers.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Header {
			return ms.headerDB[blockID]
		},
		func(blockID flow.Identifier) error {
			_, exists := ms.headerDB[blockID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	// set up local module mock
	ms.me = &module.Local{}
	ms.me.On("NodeID").Return(
		func() flow.Identifier {
			return ms.myID
		},
	)

	// set up protocol state mock
	ms.state = &protocol.State{}
	ms.state.On("Final").Return(
		func() protint.Snapshot {
			return ms.snapshot
		},
	)
	ms.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) protint.Snapshot {
			return ms.snapshot
		},
	)
	ms.state.On("Mutate", mock.Anything).Return(
		func() protint.Mutator {
			return ms.mutator
		},
	)

	// set up protocol snapshot mock
	ms.snapshot = &protocol.Snapshot{}
	ms.snapshot.On("Identities", mock.Anything).Return(
		func(filter flow.IdentityFilter) flow.IdentityList {
			return ms.participants.Filter(filter)
		},
		nil,
	)
	ms.snapshot.On("Head").Return(
		func() *flow.Header {
			return ms.head
		},
		nil,
	)

	log := zerolog.New(os.Stderr)
	retryInterval := time.Second
	e, err := New(log, ms.net, ms.me, ms.results, ms.verifier, ms.assigner, ms.state, ms.chunks, ms.con, ms.headers, retryInterval)
	require.NoError(ms.T(), err)

	ms.e = e
}

// Happy Path: When receives a ER, and 1 chunk is assigned to me,
// it will fetch that collection and chunk data, and produces a verifiable chunk
func (ms *MatchSuite) TestChunkVerified(t *testing.T) {
	// create a execution result that assigns to me
	result, assignments := createExecutionResult(
		WithChunks(
			WithAssignee(ms.myID),
		),
	)

	// add assignments to assigner
	AddAssignments(ms.assigner, assignments)

	// block header has been received
	header := MakeBlock(3, 4)
	ms.headerDB[result.BlockID] = header

	// find the exeuction node id that created the execution result
	en := ms.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// create chunk data pack
	myChunk := WithChunkIndex(result, 0)
	chunkDataPack := FromChunk(myChunk)

	// chunk data was requested once, and return the chunk data pack when requested
	OnChunkDataRequest(ms.net).Called(1).Return(func(m *messages.ChunkDataPackRequest) error {
		err := ms.e.Process(en.ID(), chunkDataPack)
		require.NoError(t, err)
		return nil
	})

	// check verifier's method is called
	VerifierCalled(ms.verifier, func(verifiableChunks []*verification.VerifiableChunkData) {
		require.Len(t, verifiableChunks, 1)
		require.Equal(t, verification.VerifiableChunkData{
			Chunk:         WithChunkIndex(result, 0),
			Header:        header,
			Result:        result,
			ChunkDataPack: chunkDataPack,
		}, verifiableChunks[0])
	})

	// engine processes the result
	err := ms.e.Process(en.ID(), result)
	require.NoError(t, err)
}

// No assignment: When receives a ER, and no chunk is assigned to me, then I won’t fetch any collection or chunk,
// nor produce any verifiable chunk
func (ms *MatchSuite) TestNoAssignment(t *testing.T) {
	result := WithChunks(1)

	chunkIndexAssignTo(ms.assigner)(0, ms.otherID)

	en, err := ms.identities.Filter(ExecutionNode(1)).ID()
	require.NoError(t, err)

	err = ms.e.Submit(en, result)
	require.NoError(t, err)

	chunkDataPack := ForChunk(1)
	err = ms.e.Process(en, chunkDataPack)
	require.NoError(ms.T(), err)

	ms.verifier.On("Submit", mock.Anything).Return(nil).Once()
}

// Multiple Assignments: When receives a ER, and 2 chunks out of 3 are assigned to me,
// it will produce 2 verifiable chunks.
func (ms *MatchSuite) TestNoAssignment(t *testing.T) {
	result, assignments := createExecutionResult(
		WithChunks(
			WithAssignee(ms.myID),
			WithAssignee(ms.myID),
			WithAssignee(ms.otherID),
		),
	)

	AddAssignments(ms.assigner, assignments)

	en, err := findNodeByRoleAndIndex(Role.ExecutionNode, 0)
	require.NoError(t, err)

	err = ms.e.Process(en, result)
	require.NoError(t, err)

	chunkDataPack := ForChunk(1)
	err = ms.e.Process(en, chunkDataPack)
	require.NoError(ms.T(), err)

	ms.verifier.On("Submit", mock.Anything).Return(nil).Twice()
}

// Duplication: When receives 2 ER for the same block, which only has 1 chunk, only 1 verifiable chunk will be produced.
func (ms *MatchSuite) TestNoAssignment(t *testing.T) {
	result, assignments := createExecutionResult(
		WithChunks(
			WithAssignee(ms.myID),
		),
	)

	AddAssignments(ms.assigner, assignments)

	en, err := findNodeByRoleAndIndex(Role.ExecutionNode, 0)
	require.NoError(t, err)

	err = ms.e.Process(en, result)
	require.NoError(t, err)

	chunkDataPack := ForChunk(1)
	err = ms.e.Process(en, chunkDataPack)
	require.NoError(ms.T(), err)

	ms.verifier.On("Submit", mock.Anything).Return(nil).Twice()
}

// Retry: When receives 1 ER, and 1 chunk is assigned assigned to me, if max retry is 3,
// the execution node fails to return data for the first 2 requests,
// and successful to return in the 3rd try, a verifiable chunk will be produced
func (ms *MatchSuite) TestRetry(t *testing.T) {
	result, assignments := createExecutionResult(
		WithChunks(
			WithAssignee(ms.myID),
		),
	)

	AddAssignments(ms.assigner, assignments)

	en, err := findNodeByRoleAndIndex(Role.ExecutionNode, 0)
	require.NoError(t, err)

	err = ms.e.Process(en, result)
	require.NoError(t, err)

	OnChunkDataRequest(ms, func(m *messages.ChunkDataPackRequest) error {
		count, exists := ms.request[m.ChunkID]
		if !exists || count < 2 {
			return nil
		}

		chunkDataPack := ForChunk(1)
		err = ms.e.Process(en, chunkDataPack)
		require.NoError(ms.T(), err)
		return nil
	})

	ms.assertVerifiable(func(t *testing.T, verifiableChunks []*verification.VerifiableChunkData) {
		require.Len(t, verifiableChunks, 1)
	}, t)
}

// MaxRetry: When receives 1 ER, and 1 chunk is assigned assigned to me, if max retry is 2,
// and the execution node fails to return data for the first 2 requests, then no verifiable chunk will be produced
func (ms *MatchSuite) TestRetry(t *testing.T) {
	// create a execution result that assigns to me
	result, assignments := createExecutionResult(
		WithChunks(
			WithAssignee(ms.myID),
		),
	)

	// add assignments to assigner
	AddAssignments(ms.assigner, assignments)

	// find the exeuction node id that created the execution result
	en := ms.participants.Filter(filter.HasRole(flow.RoleExecution))[0]

	// proess the result
	err := ms.e.Process(en, result)
	require.NoError(t, err)

	// when the execution node receives chunk data pack request, it will
	// reject the first 3 requests
	OnChunkDataRequest(ms, func(m *messages.ChunkDataPackRequest) error {
		count, exists := ms.request[m.ChunkID]
		if !exists || count < 3 {
			return nil
		}

		chunkDataPack := ForChunk(1)
		err = ms.e.Process(en.ID(), chunkDataPack)
		require.NoError(ms.T(), err)
		return nil
	})

	// verify that the verifier engine was not called, no verifiable chunk was produced
	ms.assertVerifiable(func(t *testing.T, verifiableChunks []*verification.VerifiableChunkData) {
		require.Len(t, verifiableChunks, 0)
	}, t)
}

// Concurrency: When 2 different ER are received concurrently, chunks from both
// results will be processed

// Concurrency: When chunk data pack are sent concurrently, match engine is able to receive
// all of them, and process concurrently.
