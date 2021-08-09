package access

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	consensus_follower "github.com/onflow/flow-go/follower"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type UnstakedAccessSuite struct {
	suite.Suite

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net        *testnet.FlowNetwork
	stakedID   flow.Identifier
	unstakedID flow.Identifier
	conID      flow.Identifier
	follower   consensus_follower.ConsensusFollower
}

func TestUnstakedAccessSuite(t *testing.T) {
	suite.Run(t, new(UnstakedAccessSuite))
}

func (suite *UnstakedAccessSuite) TearDownTest() {
	// avoid nil pointer errors for skipped tests
	if suite.cancel != nil {
		defer suite.cancel()
	}
	if suite.net != nil {
		suite.net.Remove()
	}
}

func (suite *UnstakedAccessSuite) SetupTest() {
	nodeConfigs := []testnet.NodeConfig{}

	// staked access node
	suite.stakedID = unittest.IdentifierFixture()
	stakedConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithID(suite.stakedID),
		testnet.AsUnstakedNetworkParticipant(),
		testnet.WithLogLevel(zerolog.InfoLevel),
	)
	nodeConfigs = append(nodeConfigs, stakedConfig)

	// consensus node (ghost)
	suite.conID = unittest.IdentifierFixture()
	conConfig := testnet.NewNodeConfig(
		flow.RoleConsensus,
		testnet.WithID(suite.conID),
		testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.FatalLevel),
	)
	nodeConfigs = append(nodeConfigs, conConfig)

	// execution node (unused)
	exeConfig := testnet.NewNodeConfig(
		flow.RoleExecution,
		testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.FatalLevel),
	)
	nodeConfigs = append(nodeConfigs, exeConfig)

	// verification node (unused)
	verConfig := testnet.NewNodeConfig(
		flow.RoleVerification,
		testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.FatalLevel),
	)
	nodeConfigs = append(nodeConfigs, verConfig)

	// collection node (unused)
	collConfig := testnet.NewNodeConfig(
		flow.RoleCollection,
		testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.FatalLevel),
	)
	nodeConfigs = append(nodeConfigs, collConfig)

	// consensus follower
	suite.unstakedID = unittest.IdentifierFixture()
	followerConfigs := []testnet.ConsensusFollowerConfig{
		testnet.ConsensusFollowerConfig{
			nodeID:         suite.unstakedID,
			upstreamNodeID: suite.stakedID,
		},
	}

	conf := testnet.NewNetworkConfig("unstaked_node_test", nodeConfigs, followerConfigs)
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf)

	// start the network
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.net.Start(suite.ctx)
}

func (suite *UnstakedAccessSuite) TestReceiveBlocks() {
	suite.follower = suite.net.ConsensusFollowerByID(suite.unstakedID)
	// TODO
}
