package verification

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// ENs is the number of execution nodes to create
	ENs int = 2

	// VNs is the number of verification nodes to create
	VNs int = 2
)

// Suite represents a test suite evaluating the integration of the test net
// against happy path of verification nodes.
type Suite struct {
	suite.Suite
	log                     zerolog.Logger
	lib.TestnetStateTracker                      // used to track messages over testnet
	cancel                  context.CancelFunc   // used to tear down the testnet
	net                     *testnet.FlowNetwork // used to keep an instance of testnet
	nodeConfigs             []testnet.NodeConfig // used to keep configuration of nodes in testnet
	nodeIDs                 []flow.Identifier    // used to keep identifier of nodes in testnet
	ghostID                 flow.Identifier      // represents id of ghost node
	exeIDs                  flow.IdentifierList  // execution nodes list
	verIDs                  flow.IdentifierList  // verification nodes list
	PreferredUnicasts       string               // preferred unicast protocols between execution and verification nodes.
}

// Ghost returns a client to interact with the Ghost node on testnet.
func (s *Suite) Ghost() *client.GhostClient {
	client, err := s.net.ContainerByID(s.ghostID).GhostClient()
	require.NoError(s.T(), err, "could not get ghost client")
	return client
}

// AccessClient returns a client to interact with the access node api on testnet.
func (s *Suite) AccessClient() *testnet.Client {
	client, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	require.NoError(s.T(), err, "could not get access client")
	return client
}

// AccessPort returns the port number of access node api on testnet.
func (s *Suite) AccessPort() string {
	return s.net.ContainerByName(testnet.PrimaryAN).Port(testnet.GRPCPort)
}

func (s *Suite) MetricsPort() string {
	return s.net.ContainerByName("execution_1").Port(testnet.GRPCPort)
}

// SetupSuite runs a bare minimum Flow network to function correctly with the following roles:
// - Two collector nodes
// - Four consensus nodes
// - List of execution nodes
// - List of verification nodes
// - One ghost node (as an execution node)
func (s *Suite) SetupSuite() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")

	blockRateFlag := "--block-rate-delay=1ms"

	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)))

	// generate the four consensus identities
	s.nodeIDs = unittest.IdentifierListFixture(4)
	for _, nodeID := range s.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus,
			testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithAdditionalFlag("--required-verification-seal-approvals=1"),
			testnet.WithAdditionalFlag("--required-construction-seal-approvals=1"),
			testnet.WithAdditionalFlag(blockRateFlag),
		)
		s.nodeConfigs = append(s.nodeConfigs, nodeConfig)
	}

	s.log.Info().Msg("before adding verification nodes (VNs); VNs=" + fmt.Sprint(VNs))
	s.verIDs = make([]flow.Identifier, VNs)

	// generate verification nodes
	for i := range s.verIDs {
		s.verIDs[i] = unittest.IdentifierFixture()
		verConfig := testnet.NewNodeConfig(flow.RoleVerification,
			testnet.WithID(s.verIDs[i]),
			testnet.WithLogLevel(zerolog.WarnLevel),
			// only verification and execution nodes run with preferred unicast protocols
			testnet.WithAdditionalFlag(fmt.Sprintf("--preferred-unicast-protocols=%s", s.PreferredUnicasts)))
		s.log.Info().Msg("================> adding verConfig[" + fmt.Sprint(i) + "]=" + s.verIDs[i].String())
		s.nodeConfigs = append(s.nodeConfigs, verConfig)
	}
	s.log.Info().Msg("after adding verification nodes (VNs)")

	s.log.Info().Msg("before adding execution nodes (ENs); ENs=" + fmt.Sprint(ENs))
	s.exeIDs = make([]flow.Identifier, ENs)

	// generate execution nodes
	for i := range s.exeIDs {
		s.exeIDs[i] = unittest.IdentifierFixture()
		exeConfig := testnet.NewNodeConfig(flow.RoleExecution,
			testnet.WithID(s.exeIDs[i]),
			testnet.WithLogLevel(zerolog.InfoLevel),
			// only verification and execution nodes run with preferred unicast protocols
			testnet.WithAdditionalFlag(fmt.Sprintf("--preferred-unicast-protocols=%s", s.PreferredUnicasts)))
		s.nodeConfigs = append(s.nodeConfigs, exeConfig)
	}

	// generates two collection node
	coll1Config := testnet.NewNodeConfig(flow.RoleCollection,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag(blockRateFlag),
	)
	coll2Config := testnet.NewNodeConfig(flow.RoleCollection,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag(blockRateFlag),
	)
	s.nodeConfigs = append(s.nodeConfigs, coll1Config, coll2Config)

	// Ghost Node
	// the ghost node's objective is to observe the messages exchanged on the
	// system and decide to terminate the test.
	// By definition, ghost node is subscribed to all channels.
	s.ghostID = unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.ghostID),
		testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.FatalLevel))
	s.nodeConfigs = append(s.nodeConfigs, ghostConfig)

	// generates, initializes, and starts the Flow network
	netConfig := testnet.NewNetworkConfig(
		"verification_tests",
		s.nodeConfigs,
		// set long staking phase to avoid QC/DKG transactions during test run
		testnet.WithViewsInStakingAuction(10_000),
		testnet.WithViewsInEpoch(100_000),
	)

	s.net = testnet.PrepareFlowNetwork(s.T(), netConfig, flow.Localnet)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.net.Start(ctx)

	// starts tracking blocks by the ghost node
	s.Track(s.T(), ctx, s.Ghost())
}

// TearDownSuite tears down the test network of Flow
func (s *Suite) TearDownSuite() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}
