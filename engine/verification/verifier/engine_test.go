package verifier_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/model/flow"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type TestSuite struct {
	suite.Suite
	net     *module.Network
	state   *protocol.State
	ss      *protocol.Snapshot
	me      *module.Local
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

	complete := unittest.CompleteResultApprovalFixture()

	err := eng.Process(invalidID, &complete)
	assert.Error(suite.T(), err)
}

func (suite *TestSuite) TestIncorrectResult() {
	// TODO
	suite.T().Skip()
}

func (suite *TestSuite) TestVerify() {
	eng := suite.TestNewEngine()

	myID := unittest.IdentifierFixture()
	consensusNodes := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	complete := unittest.CompleteResultApprovalFixture()

	suite.me.On("NodeID").Return(myID).Once()
	suite.ss.On("Identities", testifymock.Anything).Return(consensusNodes, nil).Once()
	suite.conduit.
		On("Submit", testifymock.Anything, consensusNodes.Get(0).NodeID).
		Return(nil).
		Run(func(args testifymock.Arguments) {
			ra, ok := args[0].(*flow.ResultApproval)
			suite.Assert().True(ok)
			suite.Assert().Equal(complete.Receipt.ExecutionResult.ID(), ra.ResultApprovalBody.ExecutionResultID)
		}).
		Once()

	err := eng.Process(myID, &complete)
	suite.Assert().Nil(err)

	suite.me.AssertExpectations(suite.T())
	suite.ss.AssertExpectations(suite.T())
	suite.conduit.AssertExpectations(suite.T())
}
