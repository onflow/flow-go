package epochs_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/model/flow"
	flowmodule "github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/epochs"
	module "github.com/onflow/flow-go/module/mock"
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

	log := zerolog.New(ioutil.Discard)
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
	suite.snap.On("Phase").Return(
		func() flow.EpochPhase { return suite.phase },
		func() error { return nil },
	)

	suite.epoch = new(protocol.Epoch)
	suite.counter = rand.Uint64()

	suite.nodes = unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleCollection))
	suite.me = suite.nodes.Sample(1)[0]
	suite.local.On("NodeID").Return(func() flow.Identifier {
		return suite.me.NodeID
	})

	var err error
	assignments := unittest.ClusterAssignment(2, suite.nodes)
	suite.clustering, err = flow.NewClusterList(assignments, suite.nodes)
	suite.Require().Nil(err)

	suite.epoch.On("Counter").Return(suite.counter, nil)
	suite.epoch.On("Clustering").Return(suite.clustering, nil)
	suite.signer.On("CreateVote", mock.Anything).Return(unittest.VoteFixture(), nil)

	suite.voter = epochs.NewRootQCVoter(log, suite.local, suite.signer, suite.state, []flowmodule.QCContractClient{suite.client})
}

func TestRootQCVoter(t *testing.T) {
	suite.Run(t, new(Suite))
}

// should fail if this node isn't in any cluster next epoch
func (suite *Suite) TestNonClusterParticipant() {

	// change our identity so we aren't in the cluster assignment
	suite.me = unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	err := suite.voter.Vote(context.Background(), suite.epoch)
	fmt.Println(err)
	suite.Assert().Error(err)
}

// should fail if we are not in setup phase
func (suite *Suite) TestInvalidPhase() {

	suite.phase = flow.EpochPhaseStaking
	err := suite.voter.Vote(context.Background(), suite.epoch)
	fmt.Println(err)
	suite.Assert().Error(err)
}

// should succeed and exit if we've already voted
func (suite *Suite) TestAlreadyVoted() {

	suite.voted = true
	err := suite.voter.Vote(context.Background(), suite.epoch)
	fmt.Println(err)
	suite.Assert().Nil(err)
}

// should succeed and exit if voting succeeds
func (suite *Suite) TestVoting() {
	err := suite.voter.Vote(context.Background(), suite.epoch)
	suite.Assert().Nil(err)
}
