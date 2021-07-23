package access

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	ghostclient "github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

type UnstakedAccessSuite struct {
	suite.Suite

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net            *testnet.FlowNetwork
	unstakedGhost  *ghostclient.GhostClient
	conGhost       *ghostclient.GhostClient
	unstakedReader *ghostclient.FlowMessageStreamReader
	stakedID       flow.Identifier
	unstakedID     flow.Identifier
	conID          flow.Identifier
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
	)
	nodeConfigs = append(nodeConfigs, stakedConfig)

	// unstaked access node
	suite.unstakedID = unittest.IdentifierFixture()
	unstakedConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.AsUnstaked(),
		testnet.WithID(suite.unstakedID),
		testnet.AsGhost(),
	)
	nodeConfigs = append(nodeConfigs, unstakedConfig)

	// consensus node (ghost)
	suite.conID = unittest.IdentifierFixture()
	conConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(suite.conID), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, conConfig)

	// execution node (unused)
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution, testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, exeConfig)

	// verification node (unused)
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, verConfig)

	// collection node (unused)
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, collConfig)

	conf := testnet.NewNetworkConfig("unstaked_node_test", nodeConfigs)
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf)

	// start the network
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.net.Start(suite.ctx)

	unstakedGhost := suite.net.ContainerByID(suite.unstakedID)
	client, err := common.GetGhostClient(unstakedGhost)
	require.NoError(suite.T(), err, "could not get ghost client")
	suite.unstakedGhost = client

	conGhost := suite.net.ContainerByID(suite.conID)
	client, err = common.GetGhostClient(conGhost)
	require.NoError(suite.T(), err, "could not get ghost client")
	suite.conGhost = client

	for attempts := 0; ; attempts++ {
		reader, err := suite.unstakedGhost.Subscribe(suite.ctx)
		if err == nil {
			suite.unstakedReader = reader
			break
		}
		if attempts >= 10 {
			require.NoError(suite.T(), err, "could not subscribe to unstaked ghost (%d attempts)", attempts)
		}
	}
}

func (suite *UnstakedAccessSuite) TestReceiveBlocks() {
	// First: send new block from consensus node
	// or just send directly to AN
	// Second: check that unstaked node received it
	// This can be either calling the unstaked node api directly, or
	// Third: check that staked node has it (can do this by generate a block with reference block equal to root?)
	// Can access this via FlowNetwork.root

	// TODO: add a test to MVP-test

	block := unittest.BlockFixture()

	proposal := &messages.BlockProposal{
		Header:  block.Header,
		Payload: block.Payload,
	}

	suite.conGhost.Send(suite.ctx, engine.PushBlocks, proposal, suite.stakedID)

	waitFor := 10 * time.Second
	deadline := time.Now().Add(waitFor)
	for time.Now().Before(deadline) {

		_, msg, err := suite.unstakedReader.Next()
		suite.Require().Nil(err, "could not read next message")
		suite.T().Logf("unstaked ghost recv: %T", msg)

		suite.Assert().Equal(msg, proposal)

		// switch val := msg.(type) {
		// case *messages.BlockProposal:
		// 	suite.Assert().Equal(msg, proposal)
		// }
	}

	// chain := suite.net.Root().Header.ChainID.Chain()

	// stakedContainer := suite.net.ContainerByID(suite.stakedID)
	// stakedClient, err := testnet.NewClient(stakedContainer.Addr(testnet.AccessNodeAPIPort), chain)
	// require.NoError(suite.T(), err)

	// stakedClient.client.GetLatestBlock()

}
