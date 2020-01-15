package receipts_test

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification/receipts"
	"github.com/dapperlabs/flow-go/model/flow"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TestSuite contains the context of a verifier engine test using mocked components.
type TestSuite struct {
	suite.Suite
	net                *module.Network    // used as an instance of networking layer for the mock engine
	state              *protocol.State    // used as a mock protocol state of nodes for verification engine
	ss                 *protocol.Snapshot // used as a mock representation of the snapshot of system (part of State)
	me                 *module.Local      // used as a mock representation of the mock verification node (owning the verifier engine)
	collectionsConduit *network.Conduit   // used as a mock instance of conduit for collection channel
	receiptsConduit    *network.Conduit   // used as mock instance of conduit for execution channel
	verifierEng        *network.Engine
	blocks             *mempool.Blocks
	receipts           *mempool.Receipts
	collections        *mempool.Collections
	// resources
	collection flow.Collection
	block      flow.Block
	receipt    flow.ExecutionReceipt
}

// Invoking this method executes all TestSuite tests.
func TestVerifierEngineTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

// SetupTest initiates the test setups prior to each test.
func (suite *TestSuite) SetupTest() {
	// initializing test suite fields
	suite.state = &protocol.State{}
	suite.collectionsConduit = &network.Conduit{}
	suite.receiptsConduit = &network.Conduit{}
	suite.net = &module.Network{}
	suite.me = &module.Local{}
	suite.ss = &protocol.Snapshot{}
	suite.verifierEng = &network.Engine{}
	suite.blocks = &mempool.Blocks{}
	suite.receipts = &mempool.Receipts{}
	suite.collections = &mempool.Collections{}

	suite.collection, suite.block, suite.receipt = prepareModels()

	// mocking the network registration of the engine
	// all subsequent tests are expected to have a call on Register method
	suite.net.On("Register", uint8(engine.CollectionProvider), testifymock.Anything).
		Return(suite.collectionsConduit, nil).
		Once()
	suite.net.On("Register", uint8(engine.ReceiptProvider), testifymock.Anything).
		Return(suite.receiptsConduit, nil).
		Once()
}

// TestNewEngine verifies the establishment of the network registration upon
// creation of an instance of verifier.Engine using the New method
// It also returns an instance of new engine to be used in the later tests
func (suite *TestSuite) TestNewEngine() *receipts.Engine {
	e, err := receipts.New(zerolog.Logger{}, suite.net, suite.state, suite.me, suite.verifierEng, suite.receipts, suite.blocks, suite.collections)
	require.Nil(suite.T(), err, "could not create an engine")

	suite.net.AssertExpectations(suite.T())

	return e
}

func (suite *TestSuite) TestHandleBlock() {
	eng := suite.TestNewEngine()

	suite.receipts.On("All").Return([]*flow.ExecutionReceipt{}, nil)

	// expect that that the block be added to the mempool
	suite.blocks.On("Add", &suite.block).Return(nil).Once()

	err := eng.Process(unittest.IdentifierFixture(), &suite.block)
	suite.Assert().Nil(err)

	suite.blocks.AssertExpectations(suite.T())
}

func (suite *TestSuite) TestHandleReceipt_MissingCollection() {
	eng := suite.TestNewEngine()

	// mock the receipt coming from an execution node
	execNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	// mock the set of consensus nodes
	consNodes := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus))

	suite.state.On("Final").Return(suite.ss, nil)
	suite.ss.On("Identity", execNodeID.NodeID).Return(execNodeID, nil).Once()
	suite.ss.On("Identities", testifymock.Anything).Return(consNodes, nil)

	// we have the corresponding block, but not the collection
	suite.blocks.On("Get", suite.block.ID()).Return(&suite.block, nil)
	suite.collections.On("Has", suite.collection.ID()).Return(false)

	// expect that the receipt be added to the mempool, and return it in All
	suite.receipts.On("Add", &suite.receipt).Return(nil).Once()
	suite.receipts.On("All").Return([]*flow.ExecutionReceipt{&suite.receipt}, nil).Once()

	// expect that the collection is requested
	suite.collectionsConduit.On("Submit", genSubmitParams(testifymock.Anything, consNodes)...).Return(nil).Once()

	err := eng.Process(execNodeID.NodeID, &suite.receipt)
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
	err := eng.Process(unstakedID, &suite.receipt)
	suite.Assert().Error(err)

	// receipt should not be added
	suite.receipts.AssertNotCalled(suite.T(), "Add", &suite.receipt)

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

func (suite *TestSuite) TestHandleCollection() {
	eng := suite.TestNewEngine()

	// mock the collection coming from an collection node
	collNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))

	suite.receipts.On("All").Return([]*flow.ExecutionReceipt{}, nil)
	suite.state.On("Final").Return(suite.ss).Once()
	suite.ss.On("Identity", collNodeID.NodeID).Return(collNodeID, nil).Once()

	// expect that the collection be added to the mempool
	suite.collections.On("Add", &suite.collection).Return(nil).Once()

	err := eng.Process(collNodeID.NodeID, &suite.collection)
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

	err := eng.Process(unstakedID, &suite.collection)
	suite.Assert().Error(err)

	// should not add collection to mempool
	suite.collections.AssertNotCalled(suite.T(), "Add", &suite.collection)

	// should not call verifier
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

func (suite *TestSuite) TestHandleCollection_SenderWithWrongRole() {

	invalidRoles := []flow.Role{flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification, flow.RoleObservation}

	for _, role := range invalidRoles {
		// refresh test state in between each loop
		suite.SetupTest()
		eng := suite.TestNewEngine()

		// mock the receipt coming from the invalid role
		invalidID := unittest.IdentityFixture(unittest.WithRole(role))
		suite.state.On("Final").Return(suite.ss).Once()
		suite.ss.On("Identity", invalidID.NodeID).Return(invalidID, nil).Once()

		err := eng.Process(invalidID.NodeID, &suite.collection)
		suite.Assert().Error(err)

		// should not add collection to mempool
		suite.collections.AssertNotCalled(suite.T(), "Add", &suite.collection)
	}
}

// the verifier engine should be called when the receipt is ready after any
// new resource is received.
func (suite *TestSuite) TestVerifyReady() {

	execNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	collNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	// mock the set of consensus nodes
	consNodes := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus))

	testcases := []struct {
		resource interface{}
		from     flow.Identity
		label    string
	}{
		{
			resource: &suite.receipt,
			from:     execNodeID,
			label:    "received receipt",
		}, {
			resource: &suite.collection,
			from:     collNodeID,
			label:    "received collection",
		}, {
			resource: &suite.block,
			from:     consNodes.Get(0),
			label:    "received block",
		},
	}

	for _, testcase := range testcases {
		suite.Run(testcase.label, func() {
			suite.SetupTest()
			eng := suite.TestNewEngine()

			suite.state.On("Final").Return(suite.ss, nil)
			suite.ss.On("Identity", testcase.from.NodeID).Return(testcase.from, nil).Once()

			// allow adding the received resource to mempool
			suite.receipts.On("Add", &suite.receipt).Return(nil)
			suite.collections.On("Add", &suite.collection).Return(nil)
			suite.blocks.On("Add", &suite.block).Return(nil)

			// we have all dependencies
			suite.blocks.On("Get", suite.block.ID()).Return(&suite.block, nil)
			suite.collections.On("Has", suite.collection.ID()).Return(true)
			suite.collections.On("Get", suite.collection.ID()).Return(&suite.collection, nil)
			suite.receipts.On("All").Return([]*flow.ExecutionReceipt{&suite.receipt}, nil).Once()

			// we should call the verifier engine, as the receipt is ready for verification
			suite.verifierEng.On("ProcessLocal", testifymock.Anything).Return(nil).Once()
			// the receipt should be removed from mempool
			suite.receipts.On("Rem", suite.receipt.ID()).Return(true).Once()

			// submit the resource
			err := eng.Process(testcase.from.NodeID, testcase.resource)
			suite.Assert().Nil(err)

			suite.verifierEng.AssertExpectations(suite.T())

			// the collection should not be requested
			suite.collectionsConduit.AssertNotCalled(suite.T(), "Submit", genSubmitParams(testifymock.Anything, consNodes))
		})
	}
}

// prepareModels creates a set of models for a test case.
//
// The function creates a block containing a single collection and an
// execution receipt referencing that block/collection.
func prepareModels() (flow.Collection, flow.Block, flow.ExecutionReceipt) {
	coll := unittest.CollectionFixture(3)
	guarantee := coll.Guarantee()

	content := flow.Content{
		Identities: unittest.IdentityListFixture(32),
		Guarantees: []*flow.CollectionGuarantee{&guarantee},
	}
	payload := content.Payload()
	header := unittest.BlockHeaderFixture()
	header.PayloadHash = payload.Root()

	block := flow.Block{
		Header:  header,
		Payload: payload,
		Content: content,
	}

	chunk := flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex: 0,
		},
		Index: 0,
	}

	result := flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			BlockID: block.ID(),
			Chunks:  flow.ChunkList{Chunks: []*flow.Chunk{&chunk}},
		},
	}

	receipt := flow.ExecutionReceipt{
		ExecutionResult: result,
	}

	return coll, block, receipt
}

// genSubmitParams generates the parameters of network.Conduit.Submit method for emitting the
// result approval. On receiving a result approval and identifiers of consensus nodes, it returns
// a slice with the result approval as the first element followed by the ids of consensus nodes.
func genSubmitParams(resource interface{}, identities flow.IdentityList) []interface{} {
	// extracting mock consensus nodes IDs
	params := []interface{}{resource}
	for _, targetID := range identities.NodeIDs() {
		params = append(params, targetID)
	}
	return params
}
