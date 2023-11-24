package upgrades

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

type Suite struct {
	suite.Suite
	log zerolog.Logger
	lib.TestnetStateTracker
	cancel  context.CancelFunc
	net     *testnet.FlowNetwork
	ghostID flow.Identifier
	exe1ID  flow.Identifier
}

func (s *Suite) Ghost() *client.GhostClient {
	client, err := s.net.ContainerByID(s.ghostID).GhostClient()
	require.NoError(s.T(), err, "could not get ghost client")
	return client
}

func (s *Suite) AccessClient() *testnet.Client {
	client, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	s.NoError(err, "could not get access client")
	return client
}

func (s *Suite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	collectionConfigs := []func(*testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-proposal-duration=10ms"),
		testnet.WithLogLevel(zerolog.WarnLevel),
	}

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=10ms"),
		testnet.WithAdditionalFlag(
			fmt.Sprintf(
				"--required-verification-seal-approvals=%d",
				1,
			),
		),
		testnet.WithAdditionalFlag(
			fmt.Sprintf(
				"--required-construction-seal-approvals=%d",
				1,
			),
		),
		testnet.WithLogLevel(zerolog.WarnLevel),
	}

	// a ghost node masquerading as an access node
	s.ghostID = unittest.IdentifierFixture()
	ghostNode := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithID(s.ghostID),
		testnet.AsGhost(),
	)

	s.exe1ID = unittest.IdentifierFixture()
	confs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(
			flow.RoleExecution,
			testnet.WithLogLevel(zerolog.WarnLevel),
			testnet.WithID(s.exe1ID),
			testnet.WithAdditionalFlag("--extensive-logging=true"),
			testnet.WithAdditionalFlag("--max-graceful-stop-duration=1s"),
		),
		testnet.NewNodeConfig(
			flow.RoleExecution,
			testnet.WithLogLevel(zerolog.WarnLevel),
			testnet.WithAdditionalFlag("--max-graceful-stop-duration=1s"),
		),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(
			flow.RoleVerification,
			testnet.WithLogLevel(zerolog.WarnLevel),
		),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.WarnLevel)),
		ghostNode,
	}

	netConfig := testnet.NewNetworkConfig(
		"upgrade_tests",
		confs,
		// set long staking phase to avoid QC/DKG transactions during test run
		testnet.WithViewsInStakingAuction(10_000),
		testnet.WithViewsInEpoch(100_000),
	)
	// initialize the network
	s.net = testnet.PrepareFlowNetwork(s.T(), netConfig, flow.Localnet)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.net.Start(ctx)

	// start tracking blocks
	s.Track(s.T(), ctx, s.Ghost())
}

func (s *Suite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}
