package execution

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	sdkclient  "github.com/onflow/flow-go-sdk/client"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type PerformanceTestSuite struct {
	suite.Suite
	common.TestnetStateTracker
	cancel      context.CancelFunc
	net         *testnet.FlowNetwork
	nodeConfigs []testnet.NodeConfig
	nodeIDs     []flow.Identifier
	ghostID     flow.Identifier
	exe1ID      flow.Identifier
	verID       flow.Identifier
}

func TestPerformanceTestSuite(t *testing.T) {
	suite.Run(t, new(PerformanceTestSuite))
}

func (s *PerformanceTestSuite) Ghost() *client.GhostClient {
	ghost := s.net.ContainerByID(s.ghostID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(s.T(), err, "could not get ghost client")
	return client
}

func (s *PerformanceTestSuite) AccessClient() *testnet.Client {
	chain := s.net.Root().Header.ChainID.Chain()
	client, err := testnet.NewClient(fmt.Sprintf(":%s", s.net.AccessPorts[testnet.AccessNodeAPIPort]), chain)
	require.NoError(s.T(), err, "could not get access client")
	return client
}

func (s *PerformanceTestSuite) ExecutionClient() *testnet.Client {
	execNode := s.net.ContainerByID(s.exe1ID)
	chain := s.net.Root().Header.ChainID.Chain()
	client, err := testnet.NewClient(fmt.Sprintf(":%s", execNode.Ports[testnet.ExeNodeAPIPort]), chain)
	require.NoError(s.T(), err, "could not get execution client")
	return client
}

func (s *PerformanceTestSuite) AccessPort() string {
	return s.net.AccessPorts[testnet.AccessNodeAPIPort]
}

func (s *PerformanceTestSuite) MetricsPort() string {
	return s.net.AccessPorts[testnet.ExeNodeMetricsPort]
}

func (s *PerformanceTestSuite) SetupTest() {
	blockRateFlag := "--block-rate-delay=1ms"

	// need one access node
	s.ghostID = unittest.IdentifierFixture()
	acsConfig := testnet.NewNodeConfig(flow.RoleAccess, testnet.AsGhost(), testnet.WithID(s.ghostID))
	s.nodeConfigs = append(s.nodeConfigs, acsConfig)

	// generate the four consensus identities
	s.nodeIDs = unittest.IdentifierListFixture(3)
	for _, nodeID := range s.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
			testnet.WithAdditionalFlag(blockRateFlag),
		)
		s.nodeConfigs = append(s.nodeConfigs, nodeConfig)
	}

	// need one execution nodes
	s.exe1ID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(s.exe1ID),
		testnet.WithLogLevel(zerolog.InfoLevel))
	s.nodeConfigs = append(s.nodeConfigs, exe1Config)

	// need two collection node
	coll1Config := testnet.NewNodeConfig(flow.RoleCollection,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag(blockRateFlag),
	)
	coll2Config := testnet.NewNodeConfig(flow.RoleCollection,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag(blockRateFlag),
	)
	s.nodeConfigs = append(s.nodeConfigs, coll1Config, coll2Config)

	// add the ghost (verification) node config
	verificationID := unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithID(verificationID),
		//testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.InfoLevel))
	s.nodeConfigs = append(s.nodeConfigs, ghostConfig)

	// generate the network config
	netConfig := testnet.NewNetworkConfig("execution_tests", s.nodeConfigs)

	// initialize the network
	s.net = testnet.PrepareFlowNetwork(s.T(), netConfig)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.net.Start(ctx)

	// start tracking blocks
	s.Track(s.T(), ctx, s.Ghost())
}

func (s *PerformanceTestSuite) TearDownTest() {
	s.net.Remove()
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *PerformanceTestSuite) TestSomething() {
	accessClient, err := sdkclient.New(fmt.Sprintf(":%s", s.net.AccessPorts[testnet.AccessNodeAPIPort]), grpc.WithInsecure())
	require.NoError(s.T(), err)
	latest, err := accessClient.GetLatestBlockHeader(context.Background(), false)
	require.NoError(s.T(), err)
	fmt.Println(latest.Height)
}
