package epochs_test

import (
	"context"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/factory"
	flowmodule "github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/epochs"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	local  *module.Local
	signer *hotstuff.Signer

	client *module.QCContractClient
	voted  bool

	state *protocol.State
	snap  *protocol.Snapshot

	epoch      *protocol.Epoch
	counter    uint64
	phase      flow.EpochPhase
	nodes      flow.IdentityList
	me         *flow.Identity
	clustering flow.ClusterList // cluster assignment for epoch

	voter *epochs.RootQCVoter
}

func (suite *Suite) SetupTest() {

	log := zerolog.New(io.Discard)
	suite.local = new(module.Local)
	suite.signer = new(hotstuff.Signer)
	suite.client = new(module.QCContractClient)

	suite.voted = false
	suite.client.On("Voted", mock.Anything).Return(
		func(_ context.Context) bool { return suite.voted },
		func(_ context.Context) error { return nil },
	)
	suite.client.On("SubmitVote", mock.Anything, mock.Anything).Return(nil)

	suite.state = new(protocol.State)
	suite.snap = new(protocol.Snapshot)
	suite.state.On("Final").Return(suite.snap)
	suite.phase = flow.EpochPhaseSetup
	suite.snap.On("EpochPhase").Return(
		func() flow.EpochPhase { return suite.phase },
		func() error { return nil },
	)

	suite.epoch = new(protocol.Epoch)
	suite.counter = rand.Uint64()

	suite.nodes = unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleCollection))
	suite.me = suite.nodes[rand.Intn(len(suite.nodes))]
	suite.local.On("NodeID").Return(func() flow.Identifier {
		return suite.me.NodeID
	})

	var err error
	assignments := unittest.ClusterAssignment(2, suite.nodes.ToSkeleton())
	suite.clustering, err = factory.NewClusterList(assignments, suite.nodes.ToSkeleton())
	suite.Require().NoError(err)

	suite.epoch.On("Counter").Return(suite.counter, nil)
	suite.epoch.On("Clustering").Return(suite.clustering, nil)
	suite.signer.On("CreateVote", mock.Anything).Return(unittest.VoteFixture(), nil)

	suite.voter = epochs.NewRootQCVoter(log, suite.local, suite.signer, suite.state, []flowmodule.QCContractClient{suite.client})
}

func TestRootQCVoter(t *testing.T) {
	suite.Run(t, new(Suite))
}

// TestNonClusterParticipant should fail if this node isn't in any cluster next epoch.
func (suite *Suite) TestNonClusterParticipant() {

	// change our identity so we aren't in the cluster assignment
	suite.me = unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	err := suite.voter.Vote(context.Background(), suite.epoch)
	suite.Assert().Error(err)
	suite.Assert().True(epochs.IsClusterQCNoVoteError(err))
}

// TestInvalidPhase should fail if we are not in setup phase.
func (suite *Suite) TestInvalidPhase() {

	suite.phase = flow.EpochPhaseStaking
	err := suite.voter.Vote(context.Background(), suite.epoch)
	suite.Assert().Error(err)
	suite.Assert().True(epochs.IsClusterQCNoVoteError(err))
}

// TestAlreadyVoted should succeed and exit if we've already voted.
func (suite *Suite) TestAlreadyVoted() {

	suite.voted = true
	err := suite.voter.Vote(context.Background(), suite.epoch)
	suite.Assert().NoError(err)
}

// TestVoting should succeed and exit if voting succeeds.
func (suite *Suite) TestVoting() {
	err := suite.voter.Vote(context.Background(), suite.epoch)
	suite.Assert().NoError(err)
}

// TestCancelVoting verifies correct behaviour when the context injected into the `Vote` method is cancelled.
// The `RootQCVoter` should abort voting and quickly return an `ClusterQCNoVoteError`.
func (suite *Suite) TestCancelVoting() {
	// We emulate the case, where the `QCContractClient` always returns an expected sentinel error, indicating
	// that it could not interact with the system smart contract. To create a realistic test scenario, we cancel
	// the context injected into the voter, when it is trying to submit a vote for the first time.
	// The returned transient error will cause a retry, during which the retry logic will observe the context
	// has been cancelled and exit with the context error.
	ctxWithCancel, cancel := context.WithCancel(context.Background())
	suite.client.On("SubmitVote", mock.Anything, mock.Anything).Unset()
	suite.client.On("SubmitVote", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { cancel() }).
		Return(network.NewTransientErrorf("failed to submit transaction"))

	// The `Vote` method blocks. To avoid this test hanging in case of a bug, we call vote in a separate go routine
	voteReturned := make(chan struct{})
	go func() {
		err := suite.voter.Vote(ctxWithCancel, suite.epoch)
		suite.Assert().Error(err, "when canceling voting process, Vote method should return with an error")
		suite.Assert().ErrorIs(err, context.Canceled, "`context.Canceled` should be in the error trace")
		suite.Assert().True(epochs.IsClusterQCNoVoteError(err), "got error of unexpected type")
		close(voteReturned)
	}()

	unittest.AssertClosesBefore(suite.T(), voteReturned, time.Second, "call of `Vote` method has not returned within the test's timeout")
}
