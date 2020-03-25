package ingest_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/ingest"
	"github.com/dapperlabs/flow-go/model/chunkassignment"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/model/verification/tracker"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TestSuite contains the context of a verifier engine test using mocked components.
type TestSuite struct {
	suite.Suite
	net   *module.Network
	state *protocol.State
	ss    *protocol.Snapshot
	me    *module.Local
	// mock conduit for requesting/receiving collections
	collectionsConduit *network.Conduit
	// mock conduit for requesting/receiving chunk states
	statesConduit *network.Conduit
	// mock conduit for receiving receipts
	receiptsConduit *network.Conduit
	// mock conduit for requesting/receiving chunk data packs
	chunksConduit *network.Conduit
	// mock verifier engine, should be called when all dependent resources
	// for a receipt have been received by the ingest engine.
	verifierEng *network.Engine
	// mock mempools used by the ingest engine, valid resources should be added
	// to these when they are received from an appropriate node role.
	authReceipts         *mempool.Receipts
	pendingReceipts      *mempool.Receipts
	collections          *mempool.Collections
	chunkStates          *mempool.ChunkStates
	chunkStateTracker    *mempool.ChunkStateTrackers
	chunkDataPacks       *mempool.ChunkDataPacks
	chunkDataPackTracker *mempool.ChunkDataPackTrackers
	blockStorage         *storage.Blocks
	// resources fixtures
	collection    *flow.Collection
	block         *flow.Block
	receipt       *flow.ExecutionReceipt
	chunkState    *flow.ChunkState
	chunkDataPack *flow.ChunkDataPack
	assigner      *module.ChunkAssigner // mocks chunk assigner
}

// Invoking this method executes all TestSuite tests.
func TestReceiptsEngine(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

// SetupTest initiates the test setups prior to each test.
func (suite *TestSuite) SetupTest() {
	// initializing test suite fields
	suite.collectionsConduit = &network.Conduit{}
	suite.statesConduit = &network.Conduit{}
	suite.receiptsConduit = &network.Conduit{}
	suite.chunksConduit = &network.Conduit{}
	suite.net = &module.Network{}
	suite.verifierEng = &network.Engine{}

	suite.state = &protocol.State{}
	suite.me = &module.Local{}
	suite.ss = &protocol.Snapshot{}
	suite.blockStorage = &storage.Blocks{}
	suite.authReceipts = &mempool.Receipts{}
	suite.pendingReceipts = &mempool.Receipts{}
	suite.collections = &mempool.Collections{}
	suite.chunkStates = &mempool.ChunkStates{}
	suite.chunkStateTracker = &mempool.ChunkStateTrackers{}
	suite.chunkDataPacks = &mempool.ChunkDataPacks{}
	suite.chunkDataPackTracker = &mempool.ChunkDataPackTrackers{}
	suite.assigner = &module.ChunkAssigner{}

	completeER := unittest.CompleteExecutionResultFixture(1)
	suite.collection = completeER.Collections[0]
	suite.block = completeER.Block
	suite.receipt = completeER.Receipt
	suite.chunkState = completeER.ChunkStates[0]
	suite.chunkDataPack = completeER.ChunkDataPacks[0]

	// mocking the network registration of the engine
	// all subsequent tests are expected to have a call on Register method
	suite.net.On("Register", uint8(engine.CollectionProvider), testifymock.Anything).
		Return(suite.collectionsConduit, nil).
		Once()
	suite.net.On("Register", uint8(engine.ExecutionReceiptProvider), testifymock.Anything).
		Return(suite.receiptsConduit, nil).
		Once()
	suite.net.On("Register", uint8(engine.ExecutionStateProvider), testifymock.Anything).
		Return(suite.statesConduit, nil).
		Once()
	suite.net.On("Register", uint8(engine.ChunkDataPackProvider), testifymock.Anything).
		Return(suite.chunksConduit, nil).
		Once()
}

// TestNewEngine verifies the establishment of the network registration upon
// creation of an instance of verifier.Engine using the New method
// It also returns an instance of new engine to be used in the later tests
func (suite *TestSuite) TestNewEngine() *ingest.Engine {
	e, err := ingest.New(zerolog.Logger{},
		suite.net,
		suite.state,
		suite.me,
		suite.verifierEng,
		suite.authReceipts,
		suite.pendingReceipts,
		suite.collections,
		suite.chunkStates,
		suite.chunkStateTracker,
		suite.chunkDataPacks,
		suite.chunkDataPackTracker,
		suite.blockStorage,
		suite.assigner)
	require.Nil(suite.T(), err, "could not create an engine")

	suite.net.AssertExpectations(suite.T())

	return e
}

// TestHandleBlock passes a block to ingest engine and evaluates internal path
func (suite *TestSuite) TestHandleBlock() {
	eng := suite.TestNewEngine()

	// expects that the block be added to the mempool and block storage
	suite.blockStorage.On("Store", suite.block).Return(nil).Once()

	// expects that checkPendingChunks is executed checking for pending and authenticated receipts
	// pendingReceipts iteration on All happens twice one at handlingBlocks and one at checkPendingChunks
	suite.pendingReceipts.On("All").Return([]*flow.ExecutionReceipt{}, nil).Twice()
	suite.authReceipts.On("Add", suite.receipt).Return(nil).Once()
	suite.pendingReceipts.On("Rem", suite.receipt.ID()).Return(true).Once()
	suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{}, nil).Once()

	err := eng.Process(unittest.IdentifierFixture(), suite.block)
	suite.Assert().Nil(err)

	suite.blockStorage.AssertExpectations(suite.T())
}

// TestHandleReceipt_MissingCollection evaluates that when ingest engine has both a receipt and its block
// but not the collections, it asks for the collections through the network
func (suite *TestSuite) TestHandleReceipt_MissingCollection() {
	eng := suite.TestNewEngine()

	// mock the receipt coming from an execution node
	execIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	collIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleCollection))

	suite.state.On("Final").Return(suite.ss, nil)
	suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil).Once()
	suite.ss.On("Identity", execIdentity.NodeID).Return(execIdentity, nil).Once()
	suite.ss.On("Identities", testifymock.Anything).Return(collIdentities, nil).Twice()

	// we have the corresponding block and chunk state, but not the collection
	suite.blockStorage.On("ByID", suite.block.ID()).Return(suite.block, nil).Once()
	suite.collections.On("Has", suite.collection.ID()).Return(false).Once()
	suite.chunkStates.On("Has", suite.chunkState.ID()).Return(true).Once()
	suite.chunkStates.On("ByID", suite.chunkState.ID()).Return(suite.chunkState, nil).Once()
	suite.chunkDataPacks.On("Has", suite.chunkDataPack.ID()).Return(true).Once()
	suite.chunkDataPacks.On("ByChunkID", suite.chunkDataPack.ID()).Return(suite.chunkDataPack, nil).Once()

	// expect that we already have the receipt in the authenticated receipts mempool
	suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{suite.receipt}, nil).Once()
	suite.pendingReceipts.On("All").Return([]*flow.ExecutionReceipt{}).Once()
	suite.authReceipts.On("Add", suite.receipt).Return(nil).Once()

	// expect that the collection is requested
	suite.collectionsConduit.On("Submit", testifymock.Anything, collIdentities[0].NodeID).Return(nil).Once()

	// assigns all chunks in the receipt to this node through mocking
	a := chunkassignment.NewAssignment()
	for _, chunk := range suite.receipt.ExecutionResult.Chunks {
		a.Add(chunk, []flow.Identifier{verIdentity.NodeID})
	}
	suite.assigner.On("Assign",
		testifymock.Anything,
		testifymock.Anything,
		testifymock.Anything).Return(a, nil)
	suite.me.On("NodeID", testifymock.Anything).Return(verIdentity.NodeID)

	err := eng.Process(execIdentity.NodeID, suite.receipt)
	suite.Assert().Nil(err)

	suite.authReceipts.AssertExpectations(suite.T())
	suite.collectionsConduit.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

// TestHandleReceipt_UnstakedSender evaluates sending an execution receipt from an unstaked node
// it should go to the pending receipts and (later on) dropped from the cache
// Todo dropping unauthenticated receipts from cache
func (suite *TestSuite) TestHandleReceipt_UnstakedSender() {
	eng := suite.TestNewEngine()

	myIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	suite.me.On("NodeID").Return(myIdentity.NodeID).Once()

	// mock the receipt coming from an unstaked node
	unstakedIdentity := unittest.IdentifierFixture()
	suite.state.On("Final").Return(suite.ss).Once()
	suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil).Once()
	suite.ss.On("Identity", unstakedIdentity).Return(nil, fmt.Errorf("unstaked node")).Once()

	// receipt should go to the pending receipts mempool
	suite.pendingReceipts.On("Add", suite.receipt).Return(nil).Once()
	suite.pendingReceipts.On("All").Return([]*flow.ExecutionReceipt{suite.receipt}).Once()
	suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{}).Once()
	suite.blockStorage.On("ByID", suite.block.ID()).Return(nil, fmt.Errorf("block does not exist")).Once()

	// process should fail
	err := eng.Process(unstakedIdentity, suite.receipt)
	require.NoError(suite.T(), err)

	// receipt should not be added
	suite.authReceipts.AssertNotCalled(suite.T(), "Add", suite.receipt)

	// Todo (depends on handling edge cases): adding network call for requesting block of receipt from consensus nodes

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

func (suite *TestSuite) TestHandleReceipt_SenderWithWrongRole() {
	invalidRoles := []flow.Role{flow.RoleConsensus, flow.RoleCollection, flow.RoleVerification, flow.RoleObservation}

	for _, role := range invalidRoles {
		suite.Run(fmt.Sprintf("role: %s", role), func() {
			// refresh test state in between each loop
			suite.SetupTest()
			eng := suite.TestNewEngine()

			// mock the receipt coming from the invalid role
			invalidIdentity := unittest.IdentityFixture(unittest.WithRole(role))
			suite.state.On("Final").Return(suite.ss)
			suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil)
			suite.ss.On("Identity", invalidIdentity.NodeID).Return(invalidIdentity, nil).Once()

			receipt := unittest.ExecutionReceiptFixture()

			// process should fail
			err := eng.Process(invalidIdentity.NodeID, &receipt)
			suite.Assert().Error(err)

			// receipt should not be added
			suite.authReceipts.AssertNotCalled(suite.T(), "Add", &receipt)

			// verifier should not be called
			suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
		})

	}
}

// receive a collection without any other receipt-dependent resources
func (suite *TestSuite) TestHandleCollection() {
	eng := suite.TestNewEngine()

	// mock the collection coming from an collection node
	collIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))

	suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{}, nil)
	suite.pendingReceipts.On("All").Return([]*flow.ExecutionReceipt{}, nil)
	suite.state.On("Final").Return(suite.ss).Once()
	suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil)
	suite.ss.On("Identity", collIdentity.NodeID).Return(collIdentity, nil).Once()

	// expect that the collection be added to the mempool
	suite.collections.On("Add", suite.collection).Return(nil).Once()

	err := eng.Process(collIdentity.NodeID, suite.collection)
	suite.Assert().Nil(err)

	suite.collections.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

func (suite *TestSuite) TestHandleCollection_UnstakedSender() {
	eng := suite.TestNewEngine()

	// mock the receipt coming from an unstaked node
	unstakedIdentity := unittest.IdentifierFixture()
	suite.state.On("Final").Return(suite.ss).Once()
	suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil)
	suite.ss.On("Identity", unstakedIdentity).Return(nil, errors.New("")).Once()

	err := eng.Process(unstakedIdentity, suite.collection)
	suite.Assert().Error(err)

	// should not add collection to mempool
	suite.collections.AssertNotCalled(suite.T(), "Add", suite.collection)

	// should not call verifier
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

func (suite *TestSuite) TestHandleCollection_SenderWithWrongRole() {

	invalidRoles := []flow.Role{flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification, flow.RoleObservation}

	for _, role := range invalidRoles {
		// refresh test state in between each loop
		suite.SetupTest()
		eng := suite.TestNewEngine()

		// mock the collection coming from the invalid role
		invalidIdentity := unittest.IdentityFixture(unittest.WithRole(role))
		suite.state.On("Final").Return(suite.ss).Once()
		suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil)
		suite.ss.On("Identity", invalidIdentity.NodeID).Return(invalidIdentity, nil).Once()

		err := eng.Process(invalidIdentity.NodeID, suite.collection)
		suite.Assert().Error(err)

		// should not add collection to mempool
		suite.collections.AssertNotCalled(suite.T(), "Add", suite.collection)
	}
}

func (suite *TestSuite) TestHandleExecutionState_TrackedExecutionState() {
	eng := suite.TestNewEngine()
	stateTracker := &tracker.ChunkStateTracker{
		BlockID: suite.receipt.ExecutionResult.BlockID,
		ChunkID: suite.chunkState.ChunkID,
	}

	// mock the state coming from an execution node
	exeIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{}, nil)
	suite.pendingReceipts.On("All").Return([]*flow.ExecutionReceipt{}, nil)

	// mocks a tracker for this chunk state
	suite.chunkStateTracker.On("ByChunkID", suite.chunkState.ChunkID).Return(stateTracker, nil).Once()
	suite.state.On("Final").Return(suite.ss).Once()
	suite.state.On("AtBlockID", suite.receipt.ExecutionResult.BlockID).Return(suite.ss, nil)
	suite.ss.On("Identity", exeIdentity.NodeID).Return(exeIdentity, nil).Once()
	// mocks a removing the tracker from memory once the state is processed
	suite.chunkStateTracker.On("Rem", suite.chunkState.ChunkID).Return(true).Once()
	// expect that the state be added to the mempool
	suite.chunkStates.On("Add", suite.chunkState).Return(nil).Once()

	res := &messages.ExecutionStateResponse{
		State: *suite.chunkState,
	}

	err := eng.Process(exeIdentity.NodeID, res)
	suite.Assert().Nil(err)

	suite.chunkStates.AssertExpectations(suite.T())
	suite.chunkStateTracker.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)

}

// TestHandleExecutionState_Tracked_UnstakedSender evaluates the ingest engine against
// receiving a tracked execution stake from an unstaked sender
// process should terminate with an error
func (suite *TestSuite) TestHandleExecutionState_Tracked_UnstakedSender() {
	eng := suite.TestNewEngine()
	stateTracker := &tracker.ChunkStateTracker{
		BlockID: suite.receipt.ExecutionResult.BlockID,
		ChunkID: suite.chunkState.ChunkID,
	}

	// mock the receipt coming from an unstaked node
	unstakedIdentity := unittest.IdentifierFixture()
	suite.state.On("AtBlockID", suite.receipt.ExecutionResult.BlockID).Return(suite.ss, nil).Once()
	suite.ss.On("Identity", unstakedIdentity).Return(nil, errors.New("")).Once()

	// mocks a tracker for this chunk state
	suite.chunkStateTracker.On("ByChunkID", suite.chunkState.ChunkID).Return(stateTracker, nil).Once()

	res := &messages.ExecutionStateResponse{
		State: *suite.chunkState,
	}

	err := eng.Process(unstakedIdentity, res)
	suite.Assert().Error(err)

	suite.state.AssertExpectations(suite.T())
	suite.ss.AssertExpectations(suite.T())
	suite.chunkStateTracker.AssertExpectations(suite.T())

	// should not add the state to mempool
	suite.chunkStates.AssertNotCalled(suite.T(), "Add", suite.chunkState)
	// trackers of chunk state should not be removed
	suite.chunkStates.AssertNotCalled(suite.T(), "Rem", suite.chunkState.ChunkID)

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

func (suite *TestSuite) TestHandleExecutionState_SenderWithWrongRole() {
	invalidRoles := []flow.Role{flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification, flow.RoleObservation}

	for _, role := range invalidRoles {
		// refresh test state in between each loop
		suite.SetupTest()
		eng := suite.TestNewEngine()

		// mock the state coming from the invalid role
		invalidIdentity := unittest.IdentityFixture(unittest.WithRole(role))
		suite.state.On("Final").Return(suite.ss).Once()
		suite.state.On("AtBlockID", suite.receipt.ExecutionResult.BlockID).Return(suite.ss, nil)
		suite.ss.On("Identity", invalidIdentity.NodeID).Return(invalidIdentity, nil).Once()

		err := eng.Process(invalidIdentity.NodeID, suite.chunkState)
		suite.Assert().Error(err)

		// should not add state to mempool
		suite.chunkStates.AssertNotCalled(suite.T(), "Add", suite.chunkState)
	}
}

// TestChunkStateTracker_UntrackedChunkState tests that ingest engine process method returns an error
// if it receives a ChunkStateTracker that does not have any tracker in the engine's mempool
func (suite *TestSuite) TestChunkStateTracker_UntrackedChunkState() {
	suite.SetupTest()
	eng := suite.TestNewEngine()

	execIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	// creates a chunk fixture, its data pack, and the data pack response
	chunk := unittest.ChunkFixture()
	chunkState := flow.ChunkState{
		ChunkID:   chunk.ID(),
		Registers: flow.Ledger{},
	}
	chunkDataPackResponse := &messages.ExecutionStateResponse{State: chunkState}

	// mocks tracker to return an error for this chunk ID
	suite.chunkStateTracker.On("ByChunkID", chunkState.ChunkID).
		Return(nil, fmt.Errorf("does not exist"))
	err := eng.Process(execIdentity.NodeID, chunkDataPackResponse)

	// asserts that process of an untracked chunk state returns with an error
	suite.Assert().NotNil(err)
	suite.chunkDataPackTracker.AssertExpectations(suite.T())
}

// the verifier engine should be called when the receipt is ready regardless of
// the order in which dependent resources are received.
// TODO add mempool cleanup check after the functionality gets back
// https://github.com/dapperlabs/flow-go/issues/2750
func (suite *TestSuite) TestVerifyReady() {

	execIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	collIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	consIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	verIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	testcases := []struct {
		getResource func(*TestSuite) interface{}
		from        *flow.Identity
		label       string
	}{
		{
			getResource: func(s *TestSuite) interface{} { return s.receipt },
			from:        execIdentity,
			label:       "received receipt",
		}, {
			getResource: func(s *TestSuite) interface{} { return s.collection },
			from:        collIdentity,
			label:       "received collection",
		}, {
			getResource: func(s *TestSuite) interface{} { return s.block },
			from:        consIdentity,
			label:       "received block",
		}, {
			getResource: func(s *TestSuite) interface{} {
				return &messages.ExecutionStateResponse{
					State: *s.chunkState,
				}
			},
			from:  execIdentity,
			label: "received execution state",
		},
	}

	for _, testcase := range testcases {
		suite.Run(testcase.label, func() {
			suite.SetupTest()
			eng := suite.TestNewEngine()
			stateTracker := &tracker.ChunkStateTracker{
				BlockID: suite.receipt.ExecutionResult.BlockID,
				ChunkID: suite.chunkState.ChunkID,
			}

			suite.state.On("Final", testifymock.Anything).Return(suite.ss, nil)
			suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil)
			suite.ss.On("Identity", testcase.from.NodeID).Return(testcase.from, nil).Once()
			suite.ss.On("Identities", testifymock.Anything).Return(flow.IdentityList{verIdentity}, nil).Once()
			suite.me.On("NodeID").Return(verIdentity.NodeID)

			// allow adding the received resource to mempool
			suite.authReceipts.On("Add", suite.receipt).Return(nil)
			suite.collections.On("Add", suite.collection).Return(nil)
			suite.blockStorage.On("Store", suite.block).Return(nil)
			suite.chunkStates.On("Add", suite.chunkState).Return(nil)

			// we have all dependencies
			suite.blockStorage.On("ByID", suite.block.ID()).Return(suite.block, nil)
			suite.collections.On("Has", suite.collection.ID()).Return(true)
			suite.collections.On("ByID", suite.collection.ID()).Return(suite.collection, nil)
			suite.chunkStates.On("Has", suite.chunkState.ID()).Return(true)
			suite.chunkStates.On("ByID", suite.chunkState.ID()).Return(suite.chunkState, nil)
			suite.chunkStateTracker.On("ByChunkID", suite.chunkState.ID()).Return(stateTracker, nil)
			suite.chunkDataPacks.On("Has", suite.chunkDataPack.ID()).Return(true)
			suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{suite.receipt}, nil)
			suite.pendingReceipts.On("All").Return([]*flow.ExecutionReceipt{suite.receipt}, nil)
			suite.pendingReceipts.On("Rem", suite.receipt.ID()).Return(true)
			suite.chunkStateTracker.On("Rem", suite.chunkState.ID()).Return(true).Once()
			suite.chunkDataPacks.On("ByChunkID", suite.chunkDataPack.ID()).Return(suite.chunkDataPack, nil)

			// removing the resources for a chunk
			suite.collections.On("Rem", suite.collection.ID()).Return(true).Once()

			// we have the assignment of chunk
			a := chunkassignment.NewAssignment()
			a.Add(suite.receipt.ExecutionResult.Chunks.ByIndex(0), flow.IdentifierList{verIdentity.NodeID})
			suite.assigner.On("Assign",
				testifymock.Anything,
				testifymock.Anything,
				testifymock.Anything).Return(a, nil)

			// we should call the verifier engine, as the receipt is ready for verification
			suite.verifierEng.On("ProcessLocal", testifymock.Anything).Return(nil).Once()

			// get the resource to use from the current test suite
			received := testcase.getResource(suite)
			err := eng.Process(testcase.from.NodeID, received)
			suite.Assert().Nil(err)

			suite.verifierEng.AssertExpectations(suite.T())

			// the collection should not be requested
			suite.collectionsConduit.AssertNotCalled(suite.T(), "Submit", testifymock.Anything, collIdentity)
			// the chunk state should not be requested
			suite.statesConduit.AssertNotCalled(suite.T(), "Submit", testifymock.Anything, execIdentity)

		})
	}
}

// TestChunkDataPackTracker_UntrackedChunkDataPack tests that ingest engine process method returns an error
// if it receives a ChunkDataPackResponse that does not have any tracker in the engine's mempool
func (suite *TestSuite) TestChunkDataPackTracker_UntrackedChunkDataPack() {
	suite.SetupTest()
	eng := suite.TestNewEngine()

	execIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	// creates a chunk fixture, its data pack, and the data pack response
	chunk := unittest.ChunkFixture()
	chunkDataPack := unittest.ChunkDataPackFixture(chunk.ID())
	chunkDataPackResponse := &messages.ChunkDataPackResponse{Data: chunkDataPack}

	// mocks tracker to return an error for this chunk ID
	suite.chunkDataPackTracker.On("ByChunkID", chunkDataPack.ChunkID).
		Return(nil, fmt.Errorf("does not exist"))
	err := eng.Process(execIdentity.NodeID, chunkDataPackResponse)

	// asserts that process of an untracked chunk data pack returns with an error
	suite.Assert().NotNil(err)
	suite.chunkDataPackTracker.AssertExpectations(suite.T())
}

// TestChunkDataPackTracker_HappyPath evaluates the happy path of receiving a chunk data pack upon a request
func (suite *TestSuite) TestChunkDataPackTracker_HappyPath() {
	suite.SetupTest()
	eng := suite.TestNewEngine()

	execIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	// creates a block
	block := unittest.BlockFixture()
	// creates a chunk fixture, its data pack, and the data pack response for the block
	chunk := unittest.ChunkFixture()
	chunkDataPack := unittest.ChunkDataPackFixture(chunk.ID())
	chunkDataPackResponse := &messages.ChunkDataPackResponse{Data: chunkDataPack}

	// creates a tracker for chunk data pack that binds it to the block
	track := &tracker.ChunkDataPackTracker{
		BlockID: block.ID(),
		ChunkID: chunkDataPack.ChunkID,
	}

	// mocks tracker to return the tracker for the chunk data pack
	suite.chunkDataPackTracker.On("ByChunkID", chunkDataPack.ChunkID).Return(track, nil).Once()

	// mocks state of ingest engine to return execution node ID
	suite.state.On("AtBlockID", track.BlockID).Return(suite.ss, nil).Once()
	suite.ss.On("Identity", execIdentity.NodeID).Return(execIdentity, nil).Once()

	// chunk data pack should be successfully added to mempool and the tracker should be removed
	suite.chunkDataPacks.On("Add", &chunkDataPack).Return(nil).Once()
	suite.chunkDataPackTracker.On("Rem", chunkDataPack.ChunkID).Return(true).Once()

	// terminates call to checkPendingChunks as it is out of this test's scope
	suite.pendingReceipts.On("All").Return(nil).Once()
	suite.authReceipts.On("All").Return(nil).Once()

	err := eng.Process(execIdentity.NodeID, chunkDataPackResponse)

	// asserts that process of a tracked chunk data pack should return no error
	suite.Assert().Nil(err)
	suite.chunkDataPackTracker.AssertExpectations(suite.T())
	suite.chunkDataPacks.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())
	suite.ss.AssertExpectations(suite.T())
	suite.pendingReceipts.AssertExpectations(suite.T())
	suite.authReceipts.AssertExpectations(suite.T())
}

func TestConcurrency(t *testing.T) {
	testcases := []struct {
		erCount, // number of execution receipts
		senderCount, // number of (concurrent) senders for each execution receipt
		chunksNum int // number of chunks in each execution receipt
	}{
		{
			erCount:     1,
			senderCount: 1,
			chunksNum:   1,
		}, {
			erCount:     1,
			senderCount: 10,
			chunksNum:   1,
		},
		{
			erCount:     10,
			senderCount: 1,
			chunksNum:   1,
		},
		{
			erCount:     5,
			senderCount: 10,
			chunksNum:   1,
		},
		// multiple chunks receipts
		{
			erCount:     1,
			senderCount: 1,
			chunksNum:   5, // choosing a higher number makes the test longer and longer timeout needed
		},
		{
			erCount:     1,
			senderCount: 10,
			chunksNum:   10, // choosing a higher number makes the test longer and longer timeout needed
		},
		{
			erCount:     3,
			senderCount: 1,
			chunksNum:   5, // choosing a higher number makes the test longer and longer timeout needed
		},
		{
			erCount:     3,
			senderCount: 5,
			chunksNum:   2, // choosing a higher number makes the test longer and longer timeout needed
		},
	}

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("%d-ers/%d-senders/%d-chunks", tc.erCount, tc.senderCount, tc.chunksNum), func(t *testing.T) {
			testConcurrency(t, tc.erCount, tc.senderCount, tc.chunksNum)
		})
	}
}

func testConcurrency(t *testing.T, erCount, senderCount, chunksNum int) {
	hub := stub.NewNetworkHub()

	// creates test id for each role
	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := flow.IdentityList{colID, conID, exeID, verID}

	// new chunk assignment
	a := chunkassignment.NewAssignment()

	// create `erCount` ER fixtures that will be concurrently delivered
	ers := make([]verification.CompleteExecutionResult, 0)
	// list of assigned chunks to the verifier node
	vChunks := make([]*verification.VerifiableChunk, 0)
	// a counter to assign chunks every other one, so to check if
	// ingest only sends the assigned chunks to verifier

	for i := 0; i < erCount; i++ {
		er := unittest.CompleteExecutionResultFixture(chunksNum)
		ers = append(ers, er)
		// assigns all chunks to the verifier node
		for j, chunk := range er.Receipt.ExecutionResult.Chunks {
			a.Add(chunk, []flow.Identifier{verID.NodeID})

			var endState flow.StateCommitment
			// last chunk
			if j == len(er.Receipt.ExecutionResult.Chunks)-1 {
				endState = er.Receipt.ExecutionResult.FinalStateCommit
			} else {
				endState = er.Receipt.ExecutionResult.Chunks[j+1].StartState
			}

			vc := &verification.VerifiableChunk{
				ChunkIndex: chunk.Index,
				EndState:   endState,
				Block:      er.Block,
				Receipt:    er.Receipt,
				Collection: er.Collections[chunk.Index],
				ChunkState: er.ChunkStates[chunk.Index],
			}
			vChunks = append(vChunks, vc)
		}
	}

	// set up mock verifier engine that asserts each receipt is submitted
	// to the verifier exactly once.
	verifierEng, verifierEngWG := setupMockVerifierEng(t, vChunks)
	assigner := NewMockAssigner(verID.NodeID)
	verNode := testutil.VerificationNode(t, hub, verID, identities, assigner, testutil.WithVerifierEngine(verifierEng))

	colNode := testutil.CollectionNode(t, hub, colID, identities)

	// mock the execution node with a generic node and mocked engine
	// to handle requests for chunk state
	exeNode := testutil.GenericNode(t, hub, exeID, identities)
	setupMockExeNode(t, exeNode, verID.NodeID, ers)

	verNet, ok := hub.GetNetwork(verID.NodeID)
	assert.True(t, ok)

	// the wait group tracks goroutines for each ER sending it to VER
	var senderWG sync.WaitGroup
	senderWG.Add(erCount * senderCount)

	for _, completeER := range ers {
		for _, coll := range completeER.Collections {
			err := colNode.Collections.Store(coll)
			assert.Nil(t, err)
		}

		// spin up `senderCount` sender goroutines to mimic receiving
		// the same resource multiple times
		for i := 0; i < senderCount; i++ {
			go func(j int, id flow.Identifier, block *flow.Block, receipt *flow.ExecutionReceipt) {

				sendBlock := func() {
					_ = verNode.IngestEngine.Process(conID.NodeID, block)
				}

				sendReceipt := func() {
					_ = verNode.IngestEngine.Process(exeID.NodeID, receipt)
				}

				switch j % 2 {
				case 0:
					// block then receipt
					sendBlock()
					verNet.DeliverAll(true)
					// allow another goroutine to run before sending receipt
					time.Sleep(time.Nanosecond)
					sendReceipt()
				case 1:
					// receipt then block
					sendReceipt()
					verNet.DeliverAll(true)
					// allow another goroutine to run before sending block
					time.Sleep(time.Nanosecond)
					sendBlock()
				}

				verNet.DeliverAll(true)
				go senderWG.Done()
			}(i, completeER.Receipt.ExecutionResult.ID(), completeER.Block, completeER.Receipt)
		}
	}

	// wait for all ERs to be sent to VER
	unittest.AssertReturnsBefore(t, senderWG.Wait, 3*time.Second)
	verNet.DeliverAll(false)
	unittest.AssertReturnsBefore(t, verifierEngWG.Wait, 3*time.Second)
	verNet.DeliverAll(false)
}

// setupMockExeNode sets up a mocked execution node that responds to requests for
// chunk states. Any requests that don't correspond to an execution receipt in
// the input ers list result in the test failing.
func setupMockExeNode(t *testing.T, node mock.GenericNode, verID flow.Identifier, ers []verification.CompleteExecutionResult) {
	eng := new(network.Engine)
	conduit, err := node.Net.Register(engine.ExecutionStateProvider, eng)
	assert.Nil(t, err)
	chunksConduit, err := node.Net.Register(engine.ChunkDataPackProvider, eng)
	assert.Nil(t, err)

	eng.On("Process", verID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			if req, ok := args[1].(*messages.ExecutionStateRequest); ok {
				for _, er := range ers {
					for _, chunk := range er.Receipt.ExecutionResult.Chunks {
						if chunk.ID() == req.ChunkID {
							res := &messages.ExecutionStateResponse{
								State: *er.ChunkStates[chunk.Index],
							}
							err := conduit.Submit(res, verID)
							assert.Nil(t, err)
							return
						}
					}
				}
			} else if req, ok := args[1].(*messages.ChunkDataPackRequest); ok {
				for _, er := range ers {
					for _, chunk := range er.Receipt.ExecutionResult.Chunks {
						if chunk.ID() == req.ChunkID {
							res := &messages.ChunkDataPackResponse{
								Data: *er.ChunkDataPacks[chunk.Index],
							}
							err := chunksConduit.Submit(res, verID)
							assert.Nil(t, err)
							return
						}
					}
				}
			}
			t.Logf("invalid chunk request (%T): %v ", args[1], args[1])
			t.Fail()
		}).
		Return(nil)
}

// setupMockVerifierEng sets up a mock verifier engine that asserts that a set
// of chunks are delivered to it exactly once each.
// Returns the mock engine and a wait group that unblocks when all ERs are received.
func setupMockVerifierEng(t *testing.T, vChunks []*verification.VerifiableChunk) (*network.Engine, *sync.WaitGroup) {
	eng := new(network.Engine)

	// keep track of which verifiable chunks we have received
	receivedChunks := make(map[flow.Identifier]struct{})
	var (
		// decrement the wait group when each verifiable chunk received
		wg sync.WaitGroup
		// check one verifiable chunk at a time to ensure dupe checking works
		mu sync.Mutex
	)
	wg.Add(len(vChunks))

	eng.On("ProcessLocal", testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			mu.Lock()
			defer mu.Unlock()

			vc, ok := args[0].(*verification.VerifiableChunk)
			assert.True(t, ok)

			vID := vc.Receipt.ExecutionResult.Chunks.ByIndex(vc.ChunkIndex).ID()
			// ensure there are no dupe chunks
			_, alreadySeen := receivedChunks[vID]
			if alreadySeen {
				t.Logf("received duplicated chunk (id=%s)", vID)
				t.Fail()
				return
			}

			// ensure the received chunk matches one we expect
			for _, vc := range vChunks {
				if vc.Receipt.ExecutionResult.Chunks.ByIndex(vc.ChunkIndex).ID() == vID {
					// mark it as seen and decrement the waitgroup
					receivedChunks[vID] = struct{}{}
					wg.Done()
					return
				}
			}

			// the received chunk doesn't match any expected ERs
			t.Logf("received unexpected ER (id=%s)", vID)
			t.Fail()
		}).
		Return(nil)

	return eng, &wg
}

type MockAssigner struct {
	me flow.Identifier
}

func NewMockAssigner(id flow.Identifier) *MockAssigner {
	return &MockAssigner{me: id}
}

// Assign assigns all input chunks to the verifer node
func (m *MockAssigner) Assign(ids flow.IdentityList, chunks flow.ChunkList, rng random.Rand) (*chunkassignment.Assignment, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("assigner called with empty chunk list")
	}
	a := chunkassignment.NewAssignment()
	for _, c := range chunks {
		a.Add(c, flow.IdentifierList{m.me})
	}

	return a, nil
}
