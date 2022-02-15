package verification

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// Suite represents a test suite evaluating the integration of the test net
// against happy path of verification nodes.
type Suite struct {
	suite.Suite
	log                        zerolog.Logger
	common.TestnetStateTracker                      // used to track messages over testnet
	cancel                     context.CancelFunc   // used to tear down the testnet
	net                        *testnet.FlowNetwork // used to keep an instance of testnet
	nodeConfigs                []testnet.NodeConfig // used to keep configuration of nodes in testnet
	nodeIDs                    []flow.Identifier    // used to keep identifier of nodes in testnet
	ghostID                    flow.Identifier      // represents id of ghost node
	exe1ID                     flow.Identifier
	exe2ID                     flow.Identifier
	verID                      flow.Identifier // represents id of verification node
	PreferredUnicasts          string          // preferred unicast protocols between execution and verification nodes.
}

// Ghost returns a client to interact with the Ghost node on testnet.
func (s *Suite) Ghost() *client.GhostClient {
	ghost := s.net.ContainerByID(s.ghostID)
	cli, err := common.GetGhostClient(ghost)
	require.NoError(s.T(), err, "could not get ghost client")
	return cli
}

// AccessClient returns a client to interact with the access node api on testnet.
func (s *Suite) AccessClient() *testnet.Client {
	chain := s.net.Root().Header.ChainID.Chain()
	cli, err := testnet.NewClient(fmt.Sprintf(":%s", s.net.AccessPorts[testnet.AccessNodeAPIPort]), chain)
	require.NoError(s.T(), err, "could not get access client")
	return cli
}

// AccessPort returns the port number of access node api on testnet.
func (s *Suite) AccessPort() string {
	return s.net.AccessPorts[testnet.AccessNodeAPIPort]
}

func (s *Suite) MetricsPort() string {
	return s.net.AccessPorts[testnet.ExeNodeMetricsPort]
}

// SetupSuite runs a bare minimum Flow network to function correctly with the following roles:
// - Two collector nodes
// - Four consensus nodes
// - two execution node
// - One verification node
// - One ghost node (as an execution node)
func (s *Suite) SetupSuite() {
	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("testfile", "suite.go").
		Str("testcase", s.T().Name()).
		Logger()
	s.log = logger
	s.log.Info().Msgf("================> SetupTest")

	blockRateFlag := "--block-rate-delay=1ms"

	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)))

	// generate the four consensus identities
	s.nodeIDs = unittest.IdentifierListFixture(4)
	for _, nodeID := range s.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus,
			testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
			testnet.WithAdditionalFlag("--required-verification-seal-approvals=1"),
			testnet.WithAdditionalFlag("--required-construction-seal-approvals=1"),
			testnet.WithAdditionalFlag(blockRateFlag),
		)
		s.nodeConfigs = append(s.nodeConfigs, nodeConfig)
	}

	// generates one verification node
	s.verID = unittest.IdentifierFixture()
	verConfig := testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithID(s.verID),
		testnet.WithLogLevel(zerolog.WarnLevel),
		// only verification and execution nodes run with preferred unicast protocols
		testnet.WithAdditionalFlag(fmt.Sprintf("--preferred-unicast-protocols=%s", s.PreferredUnicasts)))
	s.nodeConfigs = append(s.nodeConfigs, verConfig)

	// generates two execution nodes
	s.exe1ID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.exe1ID),
		testnet.WithLogLevel(zerolog.InfoLevel),
		// only verification and execution nodes run with preferred unicast protocols
		testnet.WithAdditionalFlag(fmt.Sprintf("--preferred-unicast-protocols=%s", s.PreferredUnicasts)))
	s.nodeConfigs = append(s.nodeConfigs, exe1Config)

	s.exe2ID = unittest.IdentifierFixture()
	exe2Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.exe2ID),
		testnet.WithLogLevel(zerolog.InfoLevel),
		// only verification and execution nodes run with preferred unicast protocols
		testnet.WithAdditionalFlag(fmt.Sprintf("--preferred-unicast-protocols=%s", s.PreferredUnicasts)))
	s.nodeConfigs = append(s.nodeConfigs, exe2Config)

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

	s.net = testnet.PrepareFlowNetwork(s.T(), netConfig)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.net.Start(ctx)

	// starts tracking blocks by the ghost node
	s.Track(s.T(), ctx, s.Ghost())
}

// TearDownSuite tears down the test network of Flow
func (s *Suite) TearDownSuite() {
	s.log.Info().Msgf("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msgf("================> Finish TearDownTest")
}
