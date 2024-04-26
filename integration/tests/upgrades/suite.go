package upgrades

import (
	"context"
	"time"

	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/protocol_state"

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

	// Determines which kvstore version is used for root state
	KVStoreFactory func(flow.Identifier) protocol_state.KVStoreAPI
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
		testnet.WithAdditionalFlag("--hotstuff-proposal-duration=100ms"),
		testnet.WithLogLevel(zerolog.WarnLevel),
	}

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=500ms"),
		testnet.WithAdditionalFlag("--required-verification-seal-approvals=0"),
		testnet.WithAdditionalFlag("--required-construction-seal-approvals=0"),
		testnet.WithLogLevel(zerolog.InfoLevel),
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

	netConfigOpts := []testnet.NetworkConfigOpt{
		// set long staking phase to avoid QC/DKG transactions during test run
		testnet.WithViewsInStakingAuction(10_000),
		testnet.WithViewsInEpoch(100_000),
		testnet.WithEpochCommitSafetyThreshold(5),
	}
	if s.KVStoreFactory != nil {
		netConfigOpts = append(netConfigOpts, testnet.WithKVStoreFactory(s.KVStoreFactory))
	}
	netConfig := testnet.NewNetworkConfig("upgrade_tests", confs, netConfigOpts...)
	// initialize the network
	s.net = testnet.PrepareFlowNetwork(s.T(), netConfig, flow.Localnet)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.net.Start(ctx)

	// start tracking blocks
	s.Track(s.T(), ctx, s.Ghost())
}

func (s *Suite) LatestProtocolStateSnapshot() *inmem.Snapshot {
	snap, err := s.AccessClient().GetLatestProtocolSnapshot(context.Background())
	require.NoError(s.T(), err)
	return snap
}

// AwaitSnapshotAtView polls until it observes a finalized snapshot with a reference
// block greater than or equal to the input target view.
func (s *Suite) AwaitSnapshotAtView(view uint64, waitFor, tick time.Duration) (snapshot *inmem.Snapshot) {
	require.Eventually(s.T(), func() bool {
		snapshot = s.LatestProtocolStateSnapshot()
		head, err := snapshot.Head()
		require.NoError(s.T(), err)
		return head.View >= view
	}, waitFor, tick)
	return
}

func (s *Suite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}
