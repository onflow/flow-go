package execution

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine/ghost/client"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
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

func (s *Suite) Ghost() *client.GhostClient {
	ghost := s.net.ContainerByID(s.ghostID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(s.T(), err, "could not get ghost client")
	return client
}

func (s *Suite) AccessClient() *testnet.Client {
	client, err := testnet.NewClient(fmt.Sprintf(":%s", s.net.AccessPorts[testnet.AccessNodeAPIPort]))
	require.NoError(s.T(), err, "could not get access client")
	return client
}

func (s *Suite) AccessPort() string {
	return s.net.AccessPorts[testnet.AccessNodeAPIPort]
}

func (s *Suite) MetricsPort() string {
	return s.net.AccessPorts[testnet.ExeNodeMetricsPort]
}

func (s *Suite) SetupTest() {

	// need one access node
	acsConfig := testnet.NewNodeConfig(flow.RoleAccess)
	s.nodeConfigs = append(s.nodeConfigs, acsConfig)

	// generate the four consensus identities
	s.nodeIDs = unittest.IdentifierListFixture(4)
	for _, nodeID := range s.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithAdditionalFlag("--hotstuff-timeout=12s"))
		s.nodeConfigs = append(s.nodeConfigs, nodeConfig)
	}

	// need one execution nodes
	s.exe1ID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(s.exe1ID),
		testnet.WithLogLevel(zerolog.InfoLevel))
	s.nodeConfigs = append(s.nodeConfigs, exe1Config)

	// need one verification node
	// s.verID = unittest.IdentifierFixture()
	// verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithID(s.verID),
	// 	testnet.WithLogLevel(zerolog.InfoLevel))
	// s.nodeConfigs = append(s.nodeConfigs, verConfig)

	// need one collection node
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel))
	s.nodeConfigs = append(s.nodeConfigs, collConfig)

	// add the ghost node config
	s.ghostID = unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithID(s.ghostID), testnet.AsGhost(),
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

func (s *Suite) TearDownTest() {
	s.net.Remove()
	s.cancel()
}
