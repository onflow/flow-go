package ingest_test

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification/ingest"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
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
	// mock verifier engine, should be called when all dependent resources
	// for a receipt have been received by the ingest engine.
	verifierEng *network.Engine
	// mock mempools used by the ingest engine, valid resources should be added
	// to these when they are received from an appropriate node role.
	blocks      *mempool.Blocks
	receipts    *mempool.Receipts
	collections *mempool.Collections
	chunkStates *mempool.ChunkStates
	// resources fixtures
	collection *flow.Collection
	block      *flow.Block
	receipt    *flow.ExecutionReceipt
	chunkState *flow.ChunkState
}

// Invoking this method executes all TestSuite tests.
func TestReceiptsEngine(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

// SetupTest initiates the test setups prior to each test.
func (suite *TestSuite) SetupTest() {
	// initializing test suite fields
	suite.state = &protocol.State{}
	suite.collectionsConduit = &network.Conduit{}
	suite.statesConduit = &network.Conduit{}
	suite.receiptsConduit = &network.Conduit{}
	suite.net = &module.Network{}
	suite.me = &module.Local{}
	suite.ss = &protocol.Snapshot{}
	suite.verifierEng = &network.Engine{}
	suite.blocks = &mempool.Blocks{}
	suite.receipts = &mempool.Receipts{}
	suite.collections = &mempool.Collections{}
	suite.chunkStates = &mempool.ChunkStates{}

	completeER := unittest.CompleteExecutionResultFixture()
	suite.collection = completeER.Collections[0]
	suite.block = completeER.Block
	suite.receipt = completeER.Receipt
	suite.chunkState = completeER.ChunkStates[0]

	// mocking the network registration of the engine
	// all subsequent tests are expected to have a call on Register method
	suite.net.On("Register", uint8(engine.CollectionProvider), testifymock.Anything).
		Return(suite.collectionsConduit, nil).
		Once()
	suite.net.On("Register", uint8(engine.ReceiptProvider), testifymock.Anything).
		Return(suite.receiptsConduit, nil).
		Once()
	suite.net.On("Register", uint8(engine.ExecutionStateProvider), testifymock.Anything).
		Return(suite.statesConduit, nil).
		Once()
}

// TestNewEngine verifies the establishment of the network registration upon
// creation of an instance of verifier.Engine using the New method
// It also returns an instance of new engine to be used in the later tests
func (suite *TestSuite) TestNewEngine() *ingest.Engine {
	e, err := ingest.New(zerolog.Logger{}, suite.net, suite.state, suite.me, suite.verifierEng, suite.receipts, suite.blocks, suite.collections, suite.chunkStates)
	require.Nil(suite.T(), err, "could not create an engine")

	suite.net.AssertExpectations(suite.T())

	return e
}

func (suite *TestSuite) TestHandleBlock() {
	eng := suite.TestNewEngine()

	suite.receipts.On("All").Return([]*flow.ExecutionReceipt{}, nil)

	// expect that that the block be added to the mempool
	suite.blocks.On("Add", suite.block).Return(nil).Once()

	err := eng.Process(unittest.IdentifierFixture(), suite.block)
	suite.Assert().Nil(err)

	suite.blocks.AssertExpectations(suite.T())
}

func (suite *TestSuite) TestHandleReceipt_MissingCollection() {
	eng := suite.TestNewEngine()

	// mock the receipt coming from an execution node
	execNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	collNodeIDs := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleCollection))

	suite.state.On("Final").Return(suite.ss, nil)
	suite.ss.On("Identity", execNodeID.NodeID).Return(execNodeID, nil).Once()
	suite.ss.On("Identities", testifymock.Anything).Return(collNodeIDs, nil).Once()

	// we have the corresponding block and chunk state, but not the collection
	suite.blocks.On("Get", suite.block.ID()).Return(suite.block, nil)
	suite.collections.On("Has", suite.collection.ID()).Return(false)
	suite.chunkStates.On("Has", suite.chunkState.ID()).Return(true)
	suite.chunkStates.On("Get", suite.chunkState.ID()).Return(suite.chunkState, nil)

	// expect that the receipt be added to the mempool, and return it in All
	suite.receipts.On("Add", suite.receipt).Return(nil).Once()
	suite.receipts.On("All").Return([]*flow.ExecutionReceipt{suite.receipt}, nil).Once()

	// expect that the collection is requested
	suite.collectionsConduit.On("Submit", testifymock.Anything, collNodeIDs.Get(0).NodeID).Return(nil).Once()

	err := eng.Process(execNodeID.NodeID, suite.receipt)
	suite.Assert().Nil(err)

	suite.receipts.AssertExpectations(suite.T())
	suite.collectionsConduit.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

func (suite *TestSuite) TestHandleReceipt_UnstakedSender() {
	eng := suite.TestNewEngine()

	myID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	suite.me.On("NodeID").Return(myID)

	// mock the receipt coming from an unstaked node
	unstakedID := unittest.IdentifierFixture()
	suite.state.On("Final").Return(suite.ss)
	suite.ss.On("Identity", unstakedID).Return(flow.Identity{}, errors.New("")).Once()

	// process should fail
	err := eng.Process(unstakedID, suite.receipt)
	suite.Assert().Error(err)

	// receipt should not be added
	suite.receipts.AssertNotCalled(suite.T(), "Add", suite.receipt)

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
			invalidID := unittest.IdentityFixture(unittest.WithRole(role))
			suite.state.On("Final").Return(suite.ss)
			suite.ss.On("Identity", invalidID.NodeID).Return(invalidID, nil).Once()

			receipt := unittest.ExecutionReceiptFixture()

			// process should fail
			err := eng.Process(invalidID.NodeID, &receipt)
			suite.Assert().Error(err)

			// receipt should not be added
			suite.receipts.AssertNotCalled(suite.T(), "Add", &receipt)

			// verifier should not be called
			suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
		})

	}
}

// receive a collection without any other receipt-dependent resources
func (suite *TestSuite) TestHandleCollection() {
	eng := suite.TestNewEngine()

	// mock the collection coming from an collection node
	collNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))

	suite.receipts.On("All").Return([]*flow.ExecutionReceipt{}, nil)
	suite.state.On("Final").Return(suite.ss).Once()
	suite.ss.On("Identity", collNodeID.NodeID).Return(collNodeID, nil).Once()

	// expect that the collection be added to the mempool
	suite.collections.On("Add", suite.collection).Return(nil).Once()

	err := eng.Process(collNodeID.NodeID, suite.collection)
	suite.Assert().Nil(err)

	suite.collections.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

func (suite *TestSuite) TestHandleCollection_UnstakedSender() {
	eng := suite.TestNewEngine()

	// mock the receipt coming from an unstaked node
	unstakedID := unittest.IdentifierFixture()
	suite.state.On("Final").Return(suite.ss).Once()
	suite.ss.On("Identity", unstakedID).Return(flow.Identity{}, errors.New("")).Once()

	err := eng.Process(unstakedID, suite.collection)
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
		invalidID := unittest.IdentityFixture(unittest.WithRole(role))
		suite.state.On("Final").Return(suite.ss).Once()
		suite.ss.On("Identity", invalidID.NodeID).Return(invalidID, nil).Once()

		err := eng.Process(invalidID.NodeID, suite.collection)
		suite.Assert().Error(err)

		// should not add collection to mempool
		suite.collections.AssertNotCalled(suite.T(), "Add", suite.collection)
	}
}

func (suite *TestSuite) TestHandleExecutionState() {
	eng := suite.TestNewEngine()

	// mock the state coming from an execution node
	exeNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	suite.receipts.On("All").Return([]*flow.ExecutionReceipt{}, nil)
	suite.state.On("Final").Return(suite.ss).Once()
	suite.ss.On("Identity", exeNodeID.NodeID).Return(exeNodeID, nil).Once()

	// expect that the state be added to the mempool
	suite.chunkStates.On("Add", suite.chunkState).Return(nil).Once()

	res := &messages.ExecutionStateResponse{
		State: *suite.chunkState,
	}

	err := eng.Process(exeNodeID.NodeID, res)
	suite.Assert().Nil(err)

	suite.chunkStates.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

func (suite *TestSuite) TestHandleExecutionState_UnstakedSender() {
	eng := suite.TestNewEngine()

	// mock the receipt coming from an unstaked node
	unstakedID := unittest.IdentifierFixture()
	suite.state.On("Final").Return(suite.ss).Once()
	suite.ss.On("Identity", unstakedID).Return(flow.Identity{}, errors.New("")).Once()

	res := &messages.ExecutionStateResponse{
		State: *suite.chunkState,
	}

	err := eng.Process(unstakedID, res)
	suite.Assert().Error(err)

	// should not add the state to mempool
	suite.chunkStates.AssertNotCalled(suite.T(), "Add", suite.chunkState)

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
		invalidID := unittest.IdentityFixture(unittest.WithRole(role))
		suite.state.On("Final").Return(suite.ss).Once()
		suite.ss.On("Identity", invalidID.NodeID).Return(invalidID, nil).Once()

		err := eng.Process(invalidID.NodeID, suite.chunkState)
		suite.Assert().Error(err)

		// should not add state to mempool
		suite.chunkStates.AssertNotCalled(suite.T(), "Add", suite.chunkState)
	}
}

// the verifier engine should be called when the receipt is ready regardless of
// the order in which dependent resources are received.
func (suite *TestSuite) TestVerifyReady() {

	execNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	collNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	consNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))

	testcases := []struct {
		getResource func(*TestSuite) interface{}
		from        flow.Identity
		label       string
	}{
		{
			getResource: func(s *TestSuite) interface{} { return s.receipt },
			from:        execNodeID,
			label:       "received receipt",
		}, {
			getResource: func(s *TestSuite) interface{} { return s.collection },
			from:        collNodeID,
			label:       "received collection",
		}, {
			getResource: func(s *TestSuite) interface{} { return s.block },
			from:        consNodeID,
			label:       "received block",
		}, {
			getResource: func(s *TestSuite) interface{} {
				return &messages.ExecutionStateResponse{
					State: *s.chunkState,
				}
			},
			from:  execNodeID,
			label: "received execution state",
		},
	}

	for _, testcase := range testcases {
		suite.Run(testcase.label, func() {
			suite.SetupTest()
			eng := suite.TestNewEngine()

			suite.state.On("Final").Return(suite.ss, nil)
			suite.ss.On("Identity", testcase.from.NodeID).Return(testcase.from, nil).Once()

			// allow adding the received resource to mempool
			suite.receipts.On("Add", suite.receipt).Return(nil)
			suite.collections.On("Add", suite.collection).Return(nil)
			suite.blocks.On("Add", suite.block).Return(nil)
			suite.chunkStates.On("Add", suite.chunkState).Return(nil)

			// we have all dependencies
			suite.blocks.On("Get", suite.block.ID()).Return(suite.block, nil)
			suite.collections.On("Has", suite.collection.ID()).Return(true)
			suite.collections.On("Get", suite.collection.ID()).Return(suite.collection, nil)
			suite.chunkStates.On("Has", suite.chunkState.ID()).Return(true)
			suite.chunkStates.On("Get", suite.chunkState.ID()).Return(suite.chunkState, nil)
			suite.receipts.On("All").Return([]*flow.ExecutionReceipt{suite.receipt}, nil).Once()

			// we should call the verifier engine, as the receipt is ready for verification
			suite.verifierEng.On("ProcessLocal", testifymock.Anything).Return(nil).Once()
			// the receipt and all dependent resources should be removed from mempool
			suite.receipts.On("Rem", suite.receipt.ID()).Return(true).Once()
			suite.blocks.On("Rem", suite.block.ID()).Return(true).Once()
			suite.collections.On("Rem", suite.collection.ID()).Return(true).Once()
			suite.chunkStates.On("Rem", suite.chunkState.ID()).Return(true).Once()

			// get the resource to use from the current test suite
			received := testcase.getResource(suite)
			err := eng.Process(testcase.from.NodeID, received)
			suite.Assert().Nil(err)

			suite.verifierEng.AssertExpectations(suite.T())

			// the collection should not be requested
			suite.collectionsConduit.AssertNotCalled(suite.T(), "Submit", testifymock.Anything, collNodeID)
			// the chunk state should not be requested
			suite.statesConduit.AssertNotCalled(suite.T(), "Submit", testifymock.Anything, execNodeID)

			// the dependent resources should be removed from the mempool
			suite.receipts.AssertCalled(suite.T(), "Rem", suite.receipt.ID())
			suite.blocks.AssertCalled(suite.T(), "Rem", suite.block.ID())
			suite.collections.AssertCalled(suite.T(), "Rem", suite.collection.ID())
			suite.chunkStates.AssertCalled(suite.T(), "Rem", suite.chunkState.ID())
		})
	}
}

func (suite *TestSuite) TestConcurrency() {
	eng := suite.TestNewEngine()

	execNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	collNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	consNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))

	suite.state.On("Final").Return(suite.ss, nil)
	suite.ss.On("Identity", testcase.from.NodeID).Return(testcase.from, nil).Once()

	// allow adding the received resource to mempool
	suite.receipts.On("Add", suite.receipt).Return(nil)
	suite.collections.On("Add", suite.collection).Return(nil)
	suite.blocks.On("Add", suite.block).Return(nil)
	suite.chunkStates.On("Add", suite.chunkState).Return(nil)

}
