package integration

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/hotstuff"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	module "github.com/dapperlabs/flow-go/module/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// End-to-end test for the HotStuff follower.
// The HotStuff follower is a thin wrapper around the Fork component which is already tested.
// In the tests below, we mainly focus the follower correctly emitting notifications and finality assertions.
func TestFollower(t *testing.T) {
	suite.Run(t, new(FollowerSuite))
}

type FollowerSuite struct {
	suite.Suite

	queue  chan interface{} // inbound proposals data
	blocks []*flow.Header   // block proposals in the order they are 'received'

	// mocked dependencies
	me        *module.Local
	pstate    *protocol.State
	snapshot  *protocol.Snapshot
	finalizer *module.Finalizer
	verifier  *mocks.Verifier
	notifier  *mocks.FinalizationConsumer

	// main logic
	follower *hotstuff.FollowerLoop
}

func (in *FollowerSuite) SetupTest() {
	in.queue = make(chan interface{}, 1024)

	// only thing used from Local identity is NodeID
	in.me = &module.Local{}
	in.me.On("NodeID").Return(unittest.IdentifierFixture())

	// init trusted root block and QC:
	rootBlock := DefaultRoot()
	rootQC := &model.QuorumCertificate{
		View:    rootBlock.View,
		BlockID: rootBlock.ID(),
	}
	in.blocks = append(in.blocks, DefaultRoot())

	// Mocked protocol state's snapshot behaviour:
	consensusMembers := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleConsensus), unittest.WithStake(1))
	networkNodes := append(consensusMembers, unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleExecution), unittest.WithStake(100))...)
	in.snapshot = &protocol.Snapshot{}
	in.snapshot.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return networkNodes.Filter(selector)
		},
		nil,
	)
	for _, m := range consensusMembers {
		in.snapshot.On("Identity", m.NodeID).Return(m, nil)
	}

	// Mocked protocol state always return the same snapshot:
	in.pstate = &protocol.State{}
	in.pstate.On("Final").Return(in.snapshot)
	in.pstate.On("AtNumber", mock.Anything).Return(in.snapshot)
	in.pstate.On("AtBlockID", mock.Anything).Return(in.snapshot)

	// Mocked hotstuff verifier behaviour
	in.verifier = &mocks.Verifier{}
	in.verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	in.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// Mocked the finalizer module behaviour:
	in.finalizer = &module.Finalizer{}
	in.finalizer.On("MakeFinal", mock.Anything).Return(
		func(blockID flow.Identifier) error {
			return nil
		},
	)

	// Mocked Notifications:
	in.notifier = &mocks.FinalizationConsumer{}
	in.notifier.On("OnBlockIncorporated", mock.Anything).Return()
	in.notifier.On("OnFinalizedBlock", mock.Anything).Return()
	in.notifier.On("OnDoubleProposeDetected", mock.Anything).Return()

	// initialize error handling and logging
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel).With().Timestamp().Logger()

	// initialize HotStuff Follower
	var err error
	consensusMemberFilter := filter.And(filter.HasRole(flow.RoleConsensus), filter.HasStake(true))
	in.follower, err = consensus.NewFollower(log, in.pstate, in.me, in.finalizer, in.verifier, in.notifier, rootBlock, rootQC, consensusMemberFilter)
	require.NoError(in.T(), err)

	in.follower.Ready()
	fmt.Println("started Follower")
}

func (in *FollowerSuite) TestStartup() {
	fmt.Println("running first test")
}

func (in *FollowerSuite) TestStartup2() {
	fmt.Println("running second test")
}
