package verifier

import (
	"errors"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// MockTestSuite contains the context of a verifier engine test using mocked components.
type MockTestSuite struct {
	suite.Suite
	net                *module.Network    // used as an instance of networking layer for the mock engine
	state              *protocol.State    // used as a mock protocol state of nodes for verification engine
	ss                 *protocol.Snapshot // used as a mock representation of the snapshot of system (part of State)
	me                 *module.Local      // used as a mock representation of the mock verification node (owning the verifier engine)
	collectionsConduit *network.Conduit   // used as a mock instance of conduit for collection channel
	receiptsConduit    *network.Conduit   // used as mock instance of conduit for execution channel
	approvalsConduit   *network.Conduit
	blocks             *mempool.Blocks
	receipts           *mempool.Receipts
	collections        *mempool.Collections
}

// Invoking this method executes all MockTestSuite tests.
func TestVerifierEngineTestSuite(t *testing.T) {
	suite.Run(t, new(MockTestSuite))
}

// Setup initiates the test setups prior to each test.
func (suite *MockTestSuite) Setup() {
	// initializing test suite fields
	suite.state = &protocol.State{}
	suite.collectionsConduit = &network.Conduit{}
	suite.receiptsConduit = &network.Conduit{}
	suite.approvalsConduit = &network.Conduit{}
	suite.net = &module.Network{}
	suite.me = &module.Local{}
	suite.ss = &protocol.Snapshot{}
	suite.blocks = &mempool.Blocks{}
	suite.receipts = &mempool.Receipts{}
	suite.collections = &mempool.Collections{}

	// mocking the network registration of the engine
	// all subsequent tests are expected to have a call on Register method
	suite.net.On("Register", uint8(engine.CollectionProvider), testifymock.Anything).
		Return(suite.collectionsConduit, nil).
		Once()
	suite.net.On("Register", uint8(engine.ReceiptProvider), testifymock.Anything).
		Return(suite.receiptsConduit, nil).
		Once()
	suite.net.On("Register", uint8(engine.ApprovalProvider), testifymock.Anything).
		Return(suite.approvalsConduit, nil).
		Once()
}

// TestNewEngine verifies the establishment of the network registration upon
// creation of an instance of verifier.Engine using the New method
// It also returns an instance of new engine to be used in the later tests
func (suite *MockTestSuite) TestNewEngine() *Engine {
	e, err := New(zerolog.Logger{}, suite.net, suite.state, suite.me, suite.receipts, suite.blocks, suite.collections)
	require.Nil(suite.T(), err, "could not create an engine")

	suite.net.AssertExpectations(suite.T())

	return e
}

func (suite *MockTestSuite) TestHandleBlock() {
	vrfy := suite.TestNewEngine()

	block := unittest.BlockFixture()

	// expect that that the block be added to the mempool
	suite.blocks.On("Add", &block).Return(nil).Once()

	err := vrfy.Process(unittest.IdentifierFixture(), &block)
	suite.Assert().Nil(err)

	suite.blocks.AssertExpectations(suite.T())
}

func (suite *MockTestSuite) TestHandleReceipt() {
	vrfy := suite.TestNewEngine()

	// mock the receipt coming from an execution node
	execNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	// mock the set of consensus nodes
	consNodes := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus))

	suite.state.On("Final").Return(suite.ss, nil)
	suite.ss.On("Identity", execNodeID.NodeID).Return(execNodeID, nil).Once()
	suite.ss.On("Identities", testifymock.Anything).Return(consNodes, nil)
	// mock methods called in verify routine
	suite.blocks.On("Get", testifymock.Anything).Return(nil, errors.New(""))
	suite.collections.On("Has", testifymock.Anything).Return(true)

	receipt := unittest.ExecutionReceiptFixture()

	// expect that the receipt be added to the mempool
	suite.receipts.On("Add", &receipt).Return(nil).Once()

	// expect that the receipt be submitted to consensus nodes
	// TODO this will need to be changed once verifier flow is finished
	suite.approvalsConduit.On("Submit", genSubmitParams(testifymock.Anything, consNodes)...).Return(nil).Once()

	err := vrfy.Process(execNodeID.NodeID, &receipt)
	suite.Assert().Nil(err)

	suite.receipts.AssertExpectations(suite.T())
}

func (suite *MockTestSuite) TestHandleReceipt_UnstakedSender() {
	vrfy := suite.TestNewEngine()

	myID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	suite.me.On("NodeID").Return(myID)

	// mock the receipt coming from an unstaked node
	unstakedID := unittest.IdentifierFixture()
	suite.state.On("Final").Return(suite.ss)
	suite.ss.On("Identity", unstakedID).Return(flow.Identity{}, errors.New("")).Once()

	receipt := unittest.ExecutionReceiptFixture()

	// process should fail
	err := vrfy.Process(unstakedID, &receipt)
	suite.Assert().Error(err)

	// receipt should not be added
	suite.receipts.AssertNotCalled(suite.T(), "Add", &receipt)

	// approval should not be submitted
	suite.approvalsConduit.AssertNotCalled(suite.T(), "Submit", testifymock.Anything, testifymock.Anything)
}

func (suite *MockTestSuite) TestHandleReceipt_SenderWithWrongRole() {
	invalidRoles := []flow.Role{flow.RoleConsensus, flow.RoleCollection, flow.RoleVerification, flow.RoleObservation}

	for _, role := range invalidRoles {
		suite.Run(fmt.Sprintf("role: %s", role), func() {
			// refresh test state in between each loop
			suite.Setup()
			vrfy := suite.TestNewEngine()

			// mock the receipt coming from the invalid role
			invalidID := unittest.IdentityFixture(unittest.WithRole(role))
			suite.state.On("Final").Return(suite.ss)
			suite.ss.On("Identity", invalidID.NodeID).Return(invalidID, nil).Once()

			receipt := unittest.ExecutionReceiptFixture()

			// process should fail
			err := vrfy.Process(invalidID.NodeID, &receipt)
			suite.Assert().Error(err)

			// receipt should not be added
			suite.receipts.AssertNotCalled(suite.T(), "Add", &receipt)

			// approval should not be submitted
			suite.approvalsConduit.AssertNotCalled(suite.T(), "Submit", testifymock.Anything, testifymock.Anything)
		})

	}
}

func (suite *MockTestSuite) TestHandleCollection() {
	vrfy := suite.TestNewEngine()

	// mock the collection coming from an collection node
	collNodeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	suite.state.On("Final").Return(suite.ss).Once()
	suite.ss.On("Identity", collNodeID.NodeID).Return(collNodeID, nil).Once()

	coll := unittest.CollectionFixture(3)

	// expect that the collection be added to the mempool
	suite.collections.On("Add", &coll).Return(nil).Once()

	err := vrfy.Process(collNodeID.NodeID, &coll)
	suite.Assert().Nil(err)

	suite.collections.AssertExpectations(suite.T())
}

func (suite *MockTestSuite) TestHandleCollection_UnstakedSender() {
	vrfy := suite.TestNewEngine()

	// mock the receipt coming from an unstaked node
	unstakedID := unittest.IdentifierFixture()
	suite.state.On("Final").Return(suite.ss).Once()
	suite.ss.On("Identity", unstakedID).Return(flow.Identity{}, errors.New("")).Once()

	coll := unittest.CollectionFixture(3)

	err := vrfy.Process(unstakedID, &coll)
	suite.Assert().Error(err)

	suite.collections.AssertNotCalled(suite.T(), "Add", &coll)
}

func (suite *MockTestSuite) TestHandleCollection_SenderWithWrongRole() {

	invalidRoles := []flow.Role{flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification, flow.RoleObservation}

	for _, role := range invalidRoles {
		// refresh test state in between each loop
		suite.Setup()
		vrfy := suite.TestNewEngine()

		// mock the receipt coming from the invalid role
		invalidID := unittest.IdentityFixture(unittest.WithRole(role))
		suite.state.On("Final").Return(suite.ss).Once()
		suite.ss.On("Identity", invalidID.NodeID).Return(invalidID, nil).Once()

		coll := unittest.CollectionFixture(3)

		err := vrfy.Process(invalidID.NodeID, &coll)
		suite.Assert().Error(err)

		suite.collections.AssertNotCalled(suite.T(), "Add", &coll)
		suite.ss.AssertExpectations(suite.T())
	}
}

//// TestProcessLocalHappyPath covers the happy path of submitting a valid execution receipt to
//// a single verifier engine till a result approval is emitted to all the consensus nodes
//func (v *MockTestSuite) TestProcessLocalHappyPath() {
//	// creating a new engine
//	vrfy := v.TestNewEngine()
//	// mocking the identity of the verification node under test
//	myID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
//
//	// mocking state for me.NodeID for internal call in ProcessLocal method
//	v.me.On("NodeID").Return(myID.NodeID).Once()
//
//	// mocking for Final().Identities(Identity(originID)) in handleExecutionReceipt method
//	v.state.On("Final").Return(v.ss).Once()
//	v.ss.On("Identity", myID.NodeID).Return(myID, nil).Once()
//
//	//mocking state for me.NodeID for internal call in handleExecutionReceipt method
//	v.me.On("NodeID").Return(myID.NodeID).Once()
//
//	// a set of mock staked consensus nodes
//	consIDs := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus))
//
//	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus)) call in verify method
//	v.state.On("Final").Return(v.ss).Once()
//	v.ss.On("Identities", testifymock.Anything).Return(consIDs, nil)
//
//	// generating a random ER and its associated result approval
//	er := unittest.ExecutionReceiptFixture()
//	ra := unittest.ResultApprovalFixture(unittest.WithExecutionResultID(er.ID()))
//
//	// the happy path ends by the verifier engine emitting a
//	// result approval to ONLY all the consensus nodes
//	// generating and mocking parameters of Submit method
//	params := genSubmitParams(&ra, consIDs)
//	v.approvalsConduit.On("Submit", params...).
//		Return(nil).
//		Once()
//
//	// store of the engine should be empty prior to the submit
//	assert.Equal(v.T(), vrfy.pool.ResultsNum(), 0)
//
//	// emitting an execution receipt form the execution node
//	_ = vrfy.ProcessLocal(er)
//
//	// store of the engine should be of size one prior to the submit
//	assert.Equal(v.T(), vrfy.pool.ResultsNum(), 1)
//
//	vrfy.wg.Wait()
//	v.state.AssertExpectations(v.T())
//	v.con.AssertExpectations(v.T())
//	v.ss.AssertExpectations(v.T())
//	v.me.AssertExpectations(v.T())
//}
//
//// TestProcessUnhappyInput covers unhappy inputs for Process method
//// including nil event, empty event, and non-existing IDs
//func (v *MockTestSuite) TestProcessUnhappyInput() {
//	// mocking state for Final().Identity(flow.Identifier{}) call in handleExecutionReceipt
//	v.state.On("Final").Return(v.ss).Once()
//	v.ss.On("Identity", flow.Identifier{}).Return(flow.Identity{}, errors.New("non-nil")).Once()
//
//	// creating a new engine
//	vrfy := v.TestNewEngine()
//
//	// nil event
//	err := vrfy.Process(flow.Identifier{}, nil)
//	assert.NotNil(v.T(), err, "failed recognizing nil event")
//
//	// non-execution receipt event
//	err = vrfy.Process(flow.Identifier{}, new(struct{}))
//	assert.NotNil(v.T(), err, "failed recognizing non-execution receipt events")
//
//	// non-recoverable id
//	err = vrfy.Process(flow.Identifier{}, &flow.ExecutionReceipt{})
//	assert.NotNilf(v.T(), err, "broken happy path: %s", err)
//
//	// asserting the calls in unhappy path
//	<-vrfy.unit.Done()
//	v.net.AssertExpectations(v.T())
//	v.state.AssertExpectations(v.T())
//	v.ss.AssertExpectations(v.T())
//}
//
//// ConcurrencyTestSetup is a sub-test method. It is not invoked independently, rather
//// it is executed as part of other test methods.
//// It does the followings:
//// 1- creates and returns a mock verifier engine as part of return values
//// 2- creates and returns a mock staked execution node ID as part of return values
//// 3- It receives the concurrency degree as input and mocks the methods to expect calls equal to that degree
//// 4- It generates and mocks a consensus committee for the verifier engine to contact, and mocks
//// Submit method of the verifier engine.
//// 5- It generates a valid execution receipt and mocks the verifier node accept that from the generated execution node ID
//// in step (2), and emit a result approval to the consensus committee generated in step (4).
//func (v *MockTestSuite) ConcurrencyTestSetup(degree, consNum int) (*flow.Identity, *Engine, *flow.ExecutionReceipt) {
//	// creating a new engine
//	vrfy := v.TestNewEngine()
//
//	// a mock staked execution node for generating a mock execution receipt
//	exeID := flow.Identity{
//		NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
//		Address: "mock-en-address",
//		Role:    flow.RoleExecution,
//	}
//
//	// mocking state fo Final().Identity(originID) call in handleExecutionReceipt method
//	v.state.On("Final").Return(v.ss).Times(degree)
//	v.ss.On("Identity", exeID.NodeID).Return(exeID, nil).Times(degree)
//
//	consIDs := unittest.IdentityListFixture(consNum, unittest.WithRole(flow.RoleConsensus))
//
//	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus)) call in verify method
//	// since all ERs are the same, only one call for identity of consensus nodes should happen
//	v.state.On("Final").Return(v.ss).Once()
//	v.ss.On("Identities", testifymock.Anything).Return(consIDs, nil)
//
//	// generating a random execution receipt and its corresponding result approval
//	receipt := unittest.ExecutionReceiptFixture()
//	approval := unittest.ResultApprovalFixture(unittest.WithExecutionResultID(receipt.ExecutionResult.ID()))
//
//	// the happy path ends by the verifier engine emitting a
//	// result approval to ONLY all the consensus nodes
//	// since all ERs are the same, only one submission should happen
//	// generating and mocking parameters of Submit method
//	params := genSubmitParams(&approval, consIDs)
//	v.con.On("Submit", params...).
//		Return(nil).
//		Once()
//
//	return &exeID, vrfy, &receipt
//}
//
//// TestProcessHappyPathConcurrentERs covers the happy path of the verifier engine on concurrently
//// receiving a valid execution receipt several times. The execution receipts are coming from a single
//// execution node. The expected behavior is to verify only a single copy of those receipts while dismissing the rest.
//func (v *MockTestSuite) TestProcessHappyPathConcurrentERs() {
//	// ConcurrencyDegree defines the number of concurrent identical ER that are submitted to the
//	// verifier node
//	const ConcurrencyDegree = 10
//
//	// mocks an execution ID and a verifier engine
//	// also mocks the reception of 10 concurrent identical execution results
//	// as well as a random execution receipt (er) and its mocked execution receipt
//	exeID, vrfy, er := v.ConcurrencyTestSetup(ConcurrencyDegree, 100)
//
//	// emitting an execution receipt form the execution node
//	errCount := 0
//	for i := 0; i < ConcurrencyDegree; i++ {
//		err := vrfy.Process(exeID.NodeID, er)
//		if err != nil {
//			errCount++
//		}
//	}
//	// all ERs are the same, so only one of them should be processed
//	assert.Equal(v.T(), errCount, ConcurrencyDegree-1)
//
//	vrfy.wg.Wait()
//	v.con.AssertExpectations(v.T())
//	v.ss.AssertExpectations(v.T())
//	v.state.AssertExpectations(v.T())
//}
//
//// TestProcessHappyPathConcurrentERs covers the happy path of the verifier engine on concurrently
//// receiving a valid execution receipt several times each over a different threads
//// In other words, this test concerns invoking the Process method over threads
//// The expected behavior is to verify only a single copy of those receipts while dismissing the rest
//func (v *MockTestSuite) TestProcessHappyPathConcurrentERsConcurrently() {
//	// Todo this test is currently broken as it assumes the Process method of engine to
//	// be called sequentially and not over a thread
//	// We skip it as it is not required for MVP
//	// Skipping this test for now
//	v.T().SkipNow()
//
//	// ConcurrencyDegree defines the number of concurrent identical ER that are submitted to the
//	// verifier node
//	const ConcurrencyDegree = 10
//
//	// mocks an execution ID and a verifier engine
//	// also mocks the reception of 10 concurrent identical execution results
//	// as well as a random execution receipt (er) and its mocked execution receipt
//	exeID, vrfy, er := v.ConcurrencyTestSetup(ConcurrencyDegree, 100)
//
//	// emitting an execution receipt form the execution node
//	errCount := 0
//	mu := sync.Mutex{}
//	for i := 0; i < ConcurrencyDegree; i++ {
//		go func() {
//			err := vrfy.Process(exeID.NodeID, er)
//			if err != nil {
//				mu.Lock()
//				errCount++
//				mu.Unlock()
//			}
//		}()
//	}
//	// all ERs are the same, so only one of them should be processed
//	assert.Equal(v.T(), errCount, ConcurrencyDegree-1)
//
//	vrfy.wg.Wait()
//	v.con.AssertExpectations(v.T())
//	v.ss.AssertExpectations(v.T())
//	v.state.AssertExpectations(v.T())
//}
//
//// TestProcessHappyPathConcurrentDifferentERs covers the happy path of the verifier engine on concurrently
//// receiving several valid execution receipts.
//// The expected behavior is to verify all of them and emit one submission of result approval per input receipt
//func (v *MockTestSuite) TestProcessHappyPathConcurrentDifferentERs() {
//	// ConcurrencyDegree defines the number of concurrent identical ER that are submitted to the
//	// verifier node
//	const ConcurrencyDegree = 10
//
//	// creating a new engine
//	vrfy := v.TestNewEngine()
//
//	// a mock staked execution node for generating a mock execution receipt
//	exeID := flow.Identity{
//		NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
//		Address: "mock-en-address",
//		Role:    flow.RoleExecution,
//	}
//
//	// mocking state for Final().Identity(originID) in handleExecutionReceipt method
//	v.state.On("Final").Return(v.ss).Times(ConcurrencyDegree)
//	v.ss.On("Identity", exeID.NodeID).Return(exeID, nil).Times(ConcurrencyDegree)
//
//	// generating a set of mock consensus ids
//	consIDs := unittest.IdentityListFixture(100, unittest.WithRole(flow.RoleConsensus))
//
//	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus)) call in verify method
//	// since ERs distinct, distinct calls for retrieving consensus nodes identity should happen
//	v.state.On("Final").Return(v.ss).Times(ConcurrencyDegree)
//	v.ss.On("Identities", testifymock.Anything).Return(consIDs, nil).Times(ConcurrencyDegree)
//
//	testTable := [ConcurrencyDegree]struct {
//		receipt *flow.ExecutionReceipt
//		params  []interface{} // parameters of the resulted Submit method of the engine corresponding to receipt
//	}{}
//
//	// preparing the test table
//	for i := 0; i < ConcurrencyDegree; i++ {
//		// generating a random execution receipt and its corresponding result approval
//		er := verification.RandomERGen()
//		restApprov := verification.RandomRAGen(er)
//		params := genSubmitParams(restApprov, consIDs)
//		testTable[i].receipt = er
//		testTable[i].params = params
//	}
//
//	// emitting an execution receipt form the execution node
//	errCount := 0
//	for i := 0; i < ConcurrencyDegree; i++ {
//		// the happy path ends by the verifier engine emitting a
//		// result approval to ONLY all the consensus nodes
//		// since ERs distinct, distinct calls for submission should happen
//		v.con.On("Submit", testTable[i].params...).
//			Return(nil).
//			Once()
//
//		err := vrfy.Process(exeID.NodeID, testTable[i].receipt)
//		if err != nil {
//			errCount++
//		}
//	}
//	// all ERs are the same, so only one of them should be processed
//	assert.Equal(v.T(), 0, errCount)
//
//	vrfy.wg.Wait()
//	v.con.AssertExpectations(v.T())
//	v.ss.AssertExpectations(v.T())
//	v.state.AssertExpectations(v.T())
//}
//
//// TestProcessHappyPath covers the happy path of the verifier engine on receiving a valid execution receipt
//// The expected behavior is to verify the receipt and emit a result approval to all consensus nodes
//func (v *MockTestSuite) TestProcessHappyPath() {
//	// creating a new engine
//	vrfy := v.TestNewEngine()
//
//	// a mock staked execution node for generating a mock execution receipt
//	exeID := flow.Identity{
//		NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
//		Address: "mock-en-address",
//		Role:    flow.RoleExecution,
//	}
//
//	// mocking state fo Final().Identity(originID) call in handleExecutionReceipt method
//	v.state.On("Final").Return(v.ss).Once()
//	v.ss.On("Identity", exeID.NodeID).Return(exeID, nil).Once()
//
//	// generating a set of mock consensus ids
//	consIDs := generateMockConsensusIDs(100)
//
//	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus)) in verify method
//	v.state.On("Final").Return(v.ss).Once()
//	v.ss.On("Identities", testifymock.Anything).Return(consIDs, nil)
//
//	// generating a random execution receipt and its corresponding result approval
//	er := verification.RandomERGen()
//	restApprov := verification.RandomRAGen(er)
//
//	// the happy path ends by the verifier engine emitting a
//	// result approval to ONLY all the consensus nodes
//	// generating and mocking parameters of Submit method
//	params := genSubmitParams(restApprov, consIDs)
//	v.con.On("Submit", params...).
//		Return(nil).
//		Once()
//
//	// emitting an execution receipt form the execution node
//	err := vrfy.Process(exeID.NodeID, er)
//	assert.Nil(v.T(), err, "failed processing execution receipt")
//
//	vrfy.wg.Wait()
//	v.con.AssertExpectations(v.T())
//	v.ss.AssertExpectations(v.T())
//	v.state.AssertExpectations(v.T())
//}

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
