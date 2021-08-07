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

	net           *testnet.FlowNetwork
	unstakedGhost *ghostclient.GhostClient
	conGhost      *ghostclient.GhostClient
	stakedID      flow.Identifier
	unstakedID    flow.Identifier
	conID         flow.Identifier
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
		testnet.WithLogLevel(zerolog.InfoLevel)
	)
	nodeConfigs = append(nodeConfigs, stakedConfig)

	// consensus node (ghost)
	suite.conID = unittest.IdentifierFixture()
	conConfig := testnet.NewNodeConfig(
		flow.RoleConsensus, 
		testnet.WithID(suite.conID), 
		testnet.AsGhost(), 
		testnet.WithLogLevel(zerolog.FatalLevel)
	)
	nodeConfigs = append(nodeConfigs, conConfig)

	// execution node (unused)
	exeConfig := testnet.NewNodeConfig(
		flow.RoleExecution, 
		testnet.AsGhost(), 
		testnet.WithLogLevel(zerolog.FatalLevel)
	)
	nodeConfigs = append(nodeConfigs, exeConfig)

	// verification node (unused)
	verConfig := testnet.NewNodeConfig(
		flow.RoleVerification, 
		testnet.AsGhost(), 
		testnet.WithLogLevel(zerolog.FatalLevel)
	)
	nodeConfigs = append(nodeConfigs, verConfig)

	// collection node (unused)
	collConfig := testnet.NewNodeConfig(
		flow.RoleCollection, 
		testnet.AsGhost(), 
		testnet.WithLogLevel(zerolog.FatalLevel)
	)
	nodeConfigs = append(nodeConfigs, collConfig)

	conf := testnet.NewNetworkConfig("unstaked_node_test", nodeConfigs)
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf)

	suite.setupConsensusFollower()

	// start the network
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.net.Start(suite.ctx)

	conGhost := suite.net.ContainerByID(suite.conID)
	client, err = common.GetGhostClient(conGhost)
	require.NoError(suite.T(), err, "could not get ghost client")
	suite.conGhost = client
}

func (suite *UnstakedAccessSuite) setupConsensusFollower() {
	// create a temporary directory to store all bootstrapping files, these
	// will be shared between all nodes
	bootstrapDir, err := ioutil.TempDir(TmpRoot, "flow-integration-bootstrap")
	require.Nil(t, err)

	// get a temporary directory in the host. On macOS the default tmp
	// directory is NOT accessible to Docker by default, so we use /tmp
	// instead.
	tmpdir, err := ioutil.TempDir(TmpRoot, "flow-integration-node")
	if err != nil {
		return fmt.Errorf("could not get tmp dir: %w", err)
	}

	// create a directory for the node database
	dataDir := filepath.Join(tmpdir, DefaultFlowDBDir)
	err = os.Mkdir(flowDBDir, 0700)
	require.NoError(t, err)

	// create a directory for the bootstrap files
	// we create a node-specific bootstrap directory to enable testing nodes
	// bootstrapping from different root state snapshots and epochs
	followerBootstrapDir := filepath.Join(tmpdir, DefaultBootstrapDir)
	err = os.Mkdir(nodeBootstrapDir, 0700)
	require.NoError(t, err)

	// copy bootstrap files to node-specific bootstrap directory
	err = io.CopyDirectory(bootstrapDir, followerBootstrapDir)
	require.NoError(t, err)

	// consensus follower
	suite.unstakedID = unittest.IdentifierFixture()
	bindPort := testingdock.RandomPort(suite.T())
	bindAddr := fmt.Sprintf(":%v", bindPort) // TODO: verify this
	opts := []consensus_follower.Option{
		consensus_follower.WithDataDir(dataDir),
		consensus_follower.WithBootstrapDir(bootstrapDir),
	} // TODO
	upstreamANPort := suite.net.ContainerByID(suite.stakedID).Ports[testnet.UnstakedNetworkPort] // TODO
	consensus_follower.NewConsensusFollower(
		suite.unstakedID,
		suite.stakedID,
		bindAddr, 
		opts...,  
	)
}

func (suite *UnstakedAccessSuite) TestReceiveBlocks() {
	// 1. Send new block from consensus node to staked AN
	// 2. Check that unstaked AN (ghost) receives it
	// 3. Check that staked AN also processed the block. This can be done by calling the
	//    Access API on the staked AN.

	block := unittest.BlockFixture()

	proposal := &messages.BlockProposal{
		Header:  block.Header,
		Payload: block.Payload,
	}

	// Send block proposal fron consensus node to staked AN
	suite.conGhost.Send(suite.ctx, engine.PushBlocks, proposal, suite.stakedID)

	m := make(chan interface{})
	go func() {
		_, msg, err := suite.unstakedReader.Next()
		suite.Require().Nil(err, "could not read next message")
		suite.T().Logf("unstaked ghost recv: %T", msg)

		m <- msg
	}()

	// Check that the unstaked AN receives the message
	select {
	case msg := <-m:
		suite.Assert().Equal(msg, proposal)
	case <-time.After(5 * time.Second):
		suite.T().Fatal("timed out waiting for next message")
	}

	// TODO: Since the staked AN follower engine will perform validation on received blocks,
	// the following check may not work unless we send a "valid" block. In particular we will
	// probably at least need to generate a block with ParentID equal to the root block ID
	// (suite.net.Root().ID())

	// chain := suite.net.Root().Header.ChainID.Chain()

	// stakedContainer := suite.net.ContainerByID(suite.stakedID)
	// stakedClient, err := testnet.NewClient(stakedContainer.Addr(testnet.AccessNodeAPIPort), chain)
	// require.NoError(suite.T(), err)

	// suite.Assert().Equal(stakedClient.GetLatestBlockID(suite.ctx), block.ID())

}
