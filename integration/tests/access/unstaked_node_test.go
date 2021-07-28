package access

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	ghostclient "github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
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
	//nodeConfigs := []testnet.NodeConfig{}
	//
	//// staked access node
	//suite.stakedID = unittest.IdentifierFixture()
	//stakedConfig := testnet.NewNodeConfig(
	//	flow.RoleAccess,
	//	testnet.WithID(suite.stakedID),
	//	testnet.AsUnstakedNetworkParticipant(),
	//)
	//nodeConfigs = append(nodeConfigs, stakedConfig)
	//
	//// unstaked access node
	//suite.unstakedID = unittest.IdentifierFixture()
	//unstakedConfig := testnet.NewNodeConfig(
	//	flow.RoleAccess,
	//	testnet.AsUnstaked(),
	//	testnet.WithID(suite.unstakedID),
	//)
	//nodeConfigs = append(nodeConfigs, unstakedConfig)
	//
	//logOnlyFatal := testnet.WithLogLevel(zerolog.FatalLevel)
	//// consensus node (ghost)
	//suite.conID = unittest.IdentifierFixture()
	//conConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(suite.conID), logOnlyFatal)
	//nodeConfigs = append(nodeConfigs, conConfig)
	//
	//// execution node (unused)
	//exeConfig := testnet.NewNodeConfig(flow.RoleExecution, logOnlyFatal)
	//nodeConfigs = append(nodeConfigs, exeConfig)
	//
	//// verification node (unused)
	//verConfig := testnet.NewNodeConfig(flow.RoleVerification, logOnlyFatal)
	//nodeConfigs = append(nodeConfigs, verConfig)
	//
	//// collection node (unused)
	//collConfig := testnet.NewNodeConfig(flow.RoleCollection, logOnlyFatal)
	//nodeConfigs = append(nodeConfigs, collConfig)
	//
	//conf := testnet.NewNetworkConfig("unstaked_node_test", nodeConfigs)
	//suite.net = testnet.PrepareFlowNetwork(suite.T(), conf)

	conf := buildNetConfig()
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf)

	// start the network
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.net.Start(suite.ctx)

	//unstakedGhost := suite.net.ContainerByID(suite.unstakedID)
	//client, err := common.GetGhostClient(unstakedGhost)
	//require.NoError(suite.T(), err, "could not get ghost client")
	//suite.unstakedGhost = client
	//
	//conGhost := suite.net.ContainerByID(suite.conID)
	//client, err := common.GetGhostClient(conGhost)
	//require.NoError(suite.T(), err, "could not get ghost client")
	//suite.conGhost = client
	//
	//for attempts := 0; ; attempts++ {
	//	reader, err := suite.unstakedGhost.Subscribe(suite.ctx)
	//	if err == nil {
	//		suite.unstakedReader = reader
	//		break
	//	}
	//	if attempts >= 10 {
	//		require.NoError(suite.T(), err, "could not subscribe to unstaked ghost (%d attempts)", attempts)
	//	}
	//}
}

func buildNetConfig() testnet.NetworkConfig {

	logOnlyFatal := testnet.WithLogLevel(zerolog.FatalLevel)

	collectionConfigs := []func(*testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		logOnlyFatal,
	}

	consensusConfigs := append(collectionConfigs,
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		logOnlyFatal,
	)

	net := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleExecution, logOnlyFatal),
		testnet.NewNodeConfig(flow.RoleExecution, logOnlyFatal),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, logOnlyFatal),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.AsUnstakedNetworkParticipant(), testnet.WithLogLevel(zerolog.InfoLevel)),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.AsUnstaked(), testnet.WithLogLevel(zerolog.InfoLevel)),
	}

	return testnet.NewNetworkConfig("unstaked_node_test", net)
}

func (suite *UnstakedAccessSuite) TestReceiveBlocks() {
	// 1. Send new block from consensus node to staked AN
	// 2. Check that unstaked AN (ghost) receives it
	// 3. Check that staked AN also processed the block. This can be done by calling the
	//    Access API on the staked AN.

	//block := unittest.BlockFixture()
	//
	//proposal := &messages.BlockProposal{
	//	Header:  block.Header,
	//	Payload: block.Payload,
	//}

	// Send block proposal fron consensus node to staked AN
	//suite.conGhost.Send(suite.ctx, engine.PushBlocks, proposal, suite.stakedID)

	// currently the setup doesn't wait for all the nodes to have completely started
	time.Sleep(10*time.Second)

	stakedANClient, err := client.New(fmt.Sprintf(":%s", suite.net.AccessPorts[testnet.AccessNodeAPIPort]), grpc.WithInsecure())
	require.NoError(suite.T(), err)


	fmt.Println("pinging the staked AN")
	ctx := context.Background()
	err = stakedANClient.Ping(ctx)
	require.NoError(suite.T(), err)

	unstakedANClient, err := client.New(fmt.Sprintf(":%s", suite.net.AccessPorts[testnet.UnstakedAccessNodeAPIPort]), grpc.WithInsecure())
	require.NoError(suite.T(), err)

	fmt.Println("pinging the unstaked AN")
	err = unstakedANClient.Ping(ctx)
	require.NoError(suite.T(), err)

	blockFromStakedAN, err := stakedANClient.GetLatestBlock(ctx, true)
	require.NoError(suite.T(), err)
	fmt.Printf(">>>>>>>>>>>>>>>>> starting height %d\n", blockFromStakedAN.Height)

	blockFromUnstakedAN, err := unstakedANClient.GetLatestBlock(ctx, true)
	require.NoError(suite.T(), err)

	require.InDelta(suite.T(), blockFromStakedAN.Height, blockFromUnstakedAN.Height, 10)

	time.Sleep(60*time.Second)

	blockFromStakedAN, err = stakedANClient.GetLatestBlock(ctx, true)
	require.NoError(suite.T(), err)
	fmt.Printf("new height on staked AN %d\n", blockFromStakedAN.Height)

	blockFromUnstakedAN, err = unstakedANClient.GetLatestBlock(ctx, true)
	require.NoError(suite.T(), err)
	fmt.Printf("new height on unstaked AN %d\n", blockFromUnstakedAN.Height)

	require.NotZero(suite.T(), blockFromStakedAN.Height)
	require.NotZero(suite.T(), blockFromUnstakedAN.Height)
	require.InDelta(suite.T(), blockFromStakedAN.Height, blockFromUnstakedAN.Height, 10)

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
