package verifier_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/model/flow"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type TestSuite struct {
	suite.Suite
	net   *module.Network
	state *protocol.State
	ss    *protocol.Snapshot
	me    *module.Local
	// mock conduit for submitting result approvals
	conduit *network.Conduit
}

func TestVerifierEgine(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (suite *TestSuite) SetupTest() {
	suite.state = &protocol.State{}
	suite.net = &module.Network{}
	suite.me = &module.Local{}
	suite.ss = &protocol.Snapshot{}
	suite.conduit = &network.Conduit{}

	suite.net.On("Register", uint8(engine.ApprovalProvider), testifymock.Anything).
		Return(suite.conduit, nil).
		Once()

	suite.state.On("Final").Return(suite.ss)
}

func (suite *TestSuite) TestNewEngine() *verifier.Engine {
	e, err := verifier.New(zerolog.Logger{}, suite.net, suite.state, suite.me)
	require.Nil(suite.T(), err)

	suite.net.AssertExpectations(suite.T())
	return e
}

func (suite *TestSuite) TestInvalidSender() {
	eng := suite.TestNewEngine()

	myID := unittest.IdentifierFixture()
	invalidID := unittest.IdentifierFixture()

	suite.me.On("NodeID").Return(myID)

	completeRA := unittest.CompleteExecutionResultFixture()

	err := eng.Process(invalidID, &completeRA)
	assert.Error(suite.T(), err)
}

func (suite *TestSuite) TestIncorrectResult() {
	// TODO when ERs are verified
	suite.T().Skip()
}

func (suite *TestSuite) TestVerify() {
	eng := suite.TestNewEngine()

	myID := unittest.IdentifierFixture()
	consensusNodes := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	completeER := unittest.CompleteExecutionResultFixture()

	suite.me.On("NodeID").Return(myID).Once()
	suite.ss.On("Identities", testifymock.Anything).Return(consensusNodes, nil).Once()
	suite.conduit.
		On("Submit", testifymock.Anything, consensusNodes.Get(0).NodeID).
		Return(nil).
		Run(func(args testifymock.Arguments) {
			// check that the approval matches the input execution result
			ra, ok := args[0].(*flow.ResultApproval)
			suite.Assert().True(ok)
			suite.Assert().Equal(completeER.Receipt.ExecutionResult.ID(), ra.ResultApprovalBody.ExecutionResultID)
		}).
		Once()

	err := eng.Process(myID, &completeER)
	suite.Assert().Nil(err)

	suite.me.AssertExpectations(suite.T())
	suite.ss.AssertExpectations(suite.T())
	suite.conduit.AssertExpectations(suite.T())
}

// checks that an execution result received by the verification node results in:
// - request of the appropriate collection
// - formation of a complete execution result by the ingest engine
// - broadcast of a matching result approval to consensus nodes
func TestHappyPath(t *testing.T) {
	hub := stub.NewNetworkHub()

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	conIDList := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	conID := conIDList.Get(0)

	identities := flow.IdentityList{colID, conID, exeID, verID}
	genesis := flow.Genesis(identities)

	verNode := testutil.VerificationNode(t, hub, verID, genesis)
	colNode := testutil.CollectionNode(t, hub, colID, genesis)

	completeER := unittest.CompleteExecutionResultFixture()

	// mock the consensus node with a generic node and mocked engine
	conNode := testutil.GenericNode(t, hub, conID, genesis)
	conEngine := new(network.Engine)
	conEngine.On("Process", verID.NodeID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			ra, ok := args[1].(*flow.ResultApproval)
			assert.True(t, ok)
			assert.Equal(t, completeER.Receipt.ExecutionResult.ID(), ra.ResultApprovalBody.ExecutionResultID)
		}).
		Return(nil).
		Once()
	_, err := conNode.Net.Register(engine.ApprovalProvider, conEngine)
	assert.Nil(t, err)

	// assume the verification node has received the block
	err = verNode.Blocks.Add(&completeER.Block)
	assert.Nil(t, err)

	// inject the collection into the collection node mempool
	err = colNode.Collections.Store(&completeER.Collections[0])
	assert.Nil(t, err)

	// send the ER from execution to verification node
	err = verNode.ReceiptsEngine.Process(exeID.NodeID, &completeER.Receipt)
	assert.Nil(t, err)

	// the receipt should be added to the mempool
	assert.True(t, verNode.Receipts.Has(completeER.Receipt.ID()))

	// flush the collection request
	verNet, ok := hub.GetNetwork(verID.NodeID)
	assert.True(t, ok)
	verNet.FlushAll()

	// flush the collection response
	colNet, ok := hub.GetNetwork(colID.NodeID)
	assert.True(t, ok)
	colNet.FlushAll()

	// the collection should be stored in the mempool
	assert.True(t, verNode.Collections.Has(completeER.Collections[0].ID()))

	// flush the result approval broadcast
	verNet.FlushAll()

	// assert that the RA was received
	conEngine.AssertExpectations(t)
}
