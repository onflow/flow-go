package cohort3

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	badgerds "github.com/ipfs/go-ds-badger2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/metrics"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestExecutionStateSync(t *testing.T) {
	suite.Run(t, new(ExecutionStateSyncSuite))
}

type ExecutionStateSyncSuite struct {
	suite.Suite
	lib.TestnetStateTracker

	log zerolog.Logger

	bridgeID     flow.Identifier
	ghostID      flow.Identifier
	observerName string

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

func (s *ExecutionStateSyncSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.buildNetworkConfig()

	// start the network
	s.net.Start(s.ctx)

	s.Track(s.T(), s.ctx, s.Ghost())
}

func (s *ExecutionStateSyncSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msgf("================> Finish TearDownTest")
}

func (s *ExecutionStateSyncSuite) Ghost() *client.GhostClient {
	client, err := s.net.ContainerByID(s.ghostID).GhostClient()
	require.NoError(s.T(), err, "could not get ghost client")
	return client
}

func (s *ExecutionStateSyncSuite) buildNetworkConfig() {
	// access node
	s.bridgeID = unittest.IdentifierFixture()
	bridgeANConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithID(s.bridgeID),
		testnet.WithLogLevel(zerolog.InfoLevel),
		testnet.WithAdditionalFlag("--supports-observer=true"),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir)),
		testnet.WithAdditionalFlag("--execution-data-retry-delay=1s"),
		testnet.WithAdditionalFlagf("--public-network-execution-data-sync-enabled=true"),
	)

	// add the ghost (access) node config
	s.ghostID = unittest.IdentifierFixture()
	ghostNode := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithID(s.ghostID),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsGhost())

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=100ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.FatalLevel),
	}

	net := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
		bridgeANConfig,
		ghostNode,
	}

	// add the observer node config
	s.observerName = testnet.PrimaryON
	observers := []testnet.ObserverConfig{{
		ContainerName: s.observerName,
		LogLevel:      zerolog.DebugLevel,
		AdditionalFlags: []string{
			fmt.Sprintf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
			"--execution-data-sync-enabled=true",
			"--event-query-mode=execution-nodes-only",
		},
	}}

	conf := testnet.NewNetworkConfig("execution state sync test", net, testnet.WithObservers(observers...))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)
}

// TestHappyPath tests that Execution Nodes generate execution data, and Access Nodes are able to
// successfully sync the data
func (s *ExecutionStateSyncSuite) TestHappyPath() {
	// Let the network run for this many blocks
	runBlocks := uint64(60)

	// We will check that execution data was downloaded for this many blocks
	// It has to be less than runBlocks since it's not possible to see which height the AN stopped
	// downloading execution data for
	checkBlocks := runBlocks / 2

	// get the first block height
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got block height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// wait for the requested number of sealed blocks, then pause the network so we can inspect the dbs
	s.BlockState.WaitForSealed(s.T(), blockA.Header.Height+runBlocks)
	s.net.StopContainers()

	metrics := metrics.NewNoopCollector()

	// start an execution data service using the Access Node's execution data db
	an := s.net.ContainerByID(s.bridgeID)
	anEds := s.nodeExecutionDataStore(an)

	// setup storage objects needed to get the execution data id
	anDB, err := an.DB()
	require.NoError(s.T(), err, "could not open db")

	anHeaders := storage.NewHeaders(metrics, anDB)
	anResults := storage.NewExecutionResults(metrics, anDB)

	// start an execution data service using the Observer Node's execution data db
	on := s.net.ContainerByName(s.observerName)
	onEds := s.nodeExecutionDataStore(on)

	// setup storage objects needed to get the execution data id
	onDB, err := on.DB()
	require.NoError(s.T(), err, "could not open db")

	onHeaders := storage.NewHeaders(metrics, onDB)
	onResults := storage.NewExecutionResults(metrics, onDB)

	// Loop through checkBlocks and verify the execution data was downloaded correctly
	for i := blockA.Header.Height; i <= blockA.Header.Height+checkBlocks; i++ {
		// access node
		header, err := anHeaders.ByHeight(i)
		require.NoError(s.T(), err, "%s: could not get header", testnet.PrimaryAN)

		result, err := anResults.ByBlockID(header.ID())
		require.NoError(s.T(), err, "%s: could not get sealed result", testnet.PrimaryAN)

		ed, err := anEds.Get(s.ctx, result.ExecutionDataID)
		if assert.NoError(s.T(), err, "%s: could not get execution data for height %v", testnet.PrimaryAN, i) {
			s.T().Logf("%s: got execution data for height %d", testnet.PrimaryAN, i)
			assert.Equal(s.T(), header.ID(), ed.BlockID)
		}

		// observer node
		header, err = onHeaders.ByHeight(i)
		require.NoError(s.T(), err, "%s: could not get header", testnet.PrimaryON)

		result, err = onResults.ByID(result.ID())
		require.NoError(s.T(), err, "%s: could not get sealed result from ON`s storage", testnet.PrimaryON)

		ed, err = onEds.Get(s.ctx, result.ExecutionDataID)
		if assert.NoError(s.T(), err, "%s: could not get execution data for height %v", testnet.PrimaryON, i) {
			s.T().Logf("%s: got execution data for height %d", testnet.PrimaryON, i)
			assert.Equal(s.T(), header.ID(), ed.BlockID)
		}
	}
}

func (s *ExecutionStateSyncSuite) nodeExecutionDataStore(node *testnet.Container) execution_data.ExecutionDataStore {
	ds, err := badgerds.NewDatastore(filepath.Join(node.ExecutionDataDBPath(), "blobstore"), &badgerds.DefaultOptions)
	require.NoError(s.T(), err, "could not get execution datastore")

	return execution_data.NewExecutionDataStore(blobs.NewBlobstore(ds), execution_data.DefaultSerializer)
}
