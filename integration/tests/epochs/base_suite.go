// Package epochs contains common functionality for the epoch integration test suite.
// Individual tests exist in sub-directories of this: cohort1, cohort2...
// Each cohort is run as a separate, sequential CI job. Since the epoch tests are long
// and resource-heavy, we split them into several cohorts, which can be run in parallel.
//
// If a new cohort is added in the future, it must be added to:
//   - ci.yml, flaky-test-monitor.yml, bors.toml (ensure new cohort of tests is run)
//   - Makefile (include new cohort in integration-test directive, etc.)
package epochs

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/unittest"
)

// BaseSuite encapsulates common functionality for epoch integration tests.
type BaseSuite struct {
	suite.Suite
	lib.TestnetStateTracker
	cancel  context.CancelFunc
	log     zerolog.Logger
	Net     *testnet.FlowNetwork
	ghostID flow.Identifier

	Client *testnet.Client
	Ctx    context.Context

	// Epoch config (lengths in views)
	StakingAuctionLen          uint64
	DKGPhaseLen                uint64
	EpochLen                   uint64
	EpochCommitSafetyThreshold uint64
	// Whether approvals are required for sealing (we only enable for VN tests because
	// requiring approvals requires a longer DKG period to avoid flakiness)
	RequiredSealApprovals uint // defaults to 0 (no approvals required)
	// Consensus Node proposal duration
	ConsensusProposalDuration time.Duration
}

// SetupTest is run automatically by the testing framework before each test case.
func (s *BaseSuite) SetupTest() {
	if s.ConsensusProposalDuration == 0 {
		s.ConsensusProposalDuration = time.Millisecond * 250
	}

	minEpochLength := s.StakingAuctionLen + s.DKGPhaseLen*3 + 20
	// ensure epoch lengths are set correctly
	require.Greater(s.T(), s.EpochLen, minEpochLength+s.EpochCommitSafetyThreshold, "epoch too short")

	s.Ctx, s.cancel = context.WithCancel(context.Background())
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	collectionConfigs := []func(*testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-proposal-duration=100ms"),
		testnet.WithLogLevel(zerolog.WarnLevel)}

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag(fmt.Sprintf("--cruise-ctl-fallback-proposal-duration=%s", s.ConsensusProposalDuration)),
		testnet.WithAdditionalFlag("--cruise-ctl-enabled=false"), // disable cruise control for integration tests
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", s.RequiredSealApprovals)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", s.RequiredSealApprovals)),
		testnet.WithLogLevel(zerolog.ErrorLevel)}

	// a ghost node masquerading as an access node
	s.ghostID = unittest.IdentifierFixture()
	ghostNode := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithID(s.ghostID),
		testnet.AsGhost())

	confs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.ErrorLevel)),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.ErrorLevel)),
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.ErrorLevel), testnet.WithAdditionalFlag("--extensive-logging=true")),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.ErrorLevel)),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.ErrorLevel)),
		ghostNode,
	}

	netConf := testnet.NewNetworkConfigWithEpochConfig("epochs-tests", confs, s.StakingAuctionLen, s.DKGPhaseLen, s.EpochLen, s.EpochCommitSafetyThreshold)

	// initialize the network
	s.Net = testnet.PrepareFlowNetwork(s.T(), netConf, flow.Localnet)

	// start the network
	s.Net.Start(s.Ctx)

	// start tracking blocks
	s.Track(s.T(), s.Ctx, s.Ghost())

	// use AN1 for test-related queries - the AN join/leave test will replace AN2
	client, err := s.Net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	require.NoError(s.T(), err)

	s.Client = client

	// log network info periodically to aid in debugging future flaky tests
	go lib.LogStatusPeriodically(s.T(), s.Ctx, s.log, s.Client, 5*time.Second)
}

func (s *BaseSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.Net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *BaseSuite) Ghost() *client.GhostClient {
	client, err := s.Net.ContainerByID(s.ghostID).GhostClient()
	require.NoError(s.T(), err, "could not get ghost Client")
	return client
}

// TimedLogf logs the message using t.Log and the suite logger, but prefixes the current time.
// This enables viewing logs inline with Docker logs as well as other test logs.
func (s *BaseSuite) TimedLogf(msg string, args ...interface{}) {
	s.log.Info().Msgf(msg, args...)
	args = append([]interface{}{time.Now().String()}, args...)
	s.T().Logf("%s - "+msg, args...)
}

// AwaitEpochPhase waits for the given phase, in the given epoch.
func (s *BaseSuite) AwaitEpochPhase(ctx context.Context, expectedEpoch uint64, expectedPhase flow.EpochPhase, waitFor, tick time.Duration) {
	var actualEpoch uint64
	var actualPhase flow.EpochPhase
	condition := func() bool {
		snapshot, err := s.Client.GetLatestProtocolSnapshot(ctx)
		require.NoError(s.T(), err)

		actualEpoch, err = snapshot.Epochs().Current().Counter()
		require.NoError(s.T(), err)
		actualPhase, err = snapshot.EpochPhase()
		require.NoError(s.T(), err)

		return actualEpoch == expectedEpoch && actualPhase == expectedPhase
	}
	require.Eventuallyf(s.T(), condition, waitFor, tick, "did not reach expectedEpoch %d phase %s within %s. Last saw epoch=%d and phase=%s", expectedEpoch, expectedPhase, waitFor, actualEpoch, actualPhase)
}

// GetContainersByRole returns all containers from the network for the specified role, making sure the containers are not ghost nodes.
func (s *BaseSuite) GetContainersByRole(role flow.Role) []*testnet.Container {
	nodes := s.Net.ContainersByRole(role, false)
	require.True(s.T(), len(nodes) > 0)
	return nodes
}

// AwaitFinalizedView polls until it observes that the latest finalized block has a view
// greater than or equal to the input view. This is used to wait until when an epoch
// transition must have happened.
func (s *BaseSuite) AwaitFinalizedView(ctx context.Context, view uint64, waitFor, tick time.Duration) {
	require.Eventually(s.T(), func() bool {
		finalized := s.GetLatestFinalizedHeader(ctx)
		return finalized.View >= view
	}, waitFor, tick)
}

// GetLatestFinalizedHeader retrieves the latest finalized block, as reported in LatestSnapshot.
func (s *BaseSuite) GetLatestFinalizedHeader(ctx context.Context) *flow.Header {
	snapshot := s.GetLatestProtocolSnapshot(ctx)
	finalized, err := snapshot.Head()
	require.NoError(s.T(), err)
	return finalized
}

// AssertInEpoch requires actual epoch counter is equal to counter provided.
func (s *BaseSuite) AssertInEpoch(ctx context.Context, expectedEpoch uint64) {
	actualEpoch := s.CurrentEpoch(ctx)
	require.Equalf(s.T(), expectedEpoch, actualEpoch, "expected to be in epoch %d got %d", expectedEpoch, actualEpoch)
}

// CurrentEpoch returns the current epoch counter.
func (s *BaseSuite) CurrentEpoch(ctx context.Context) uint64 {
	snapshot := s.GetLatestProtocolSnapshot(ctx)
	counter, err := snapshot.Epochs().Current().Counter()
	require.NoError(s.T(), err)
	return counter
}

// GetLatestProtocolSnapshot returns the latest protocol snapshot.
func (s *BaseSuite) GetLatestProtocolSnapshot(ctx context.Context) *inmem.Snapshot {
	snapshot, err := s.Client.GetLatestProtocolSnapshot(ctx)
	require.NoError(s.T(), err)
	return snapshot
}
