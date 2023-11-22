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

	bridgeID flow.Identifier
	ghostID  flow.Identifier

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
		testnet.WithLogLevel(zerolog.DebugLevel),
		testnet.WithAdditionalFlag("--supports-observer=true"),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir)),
		testnet.WithAdditionalFlag("--execution-data-retry-delay=1s"),
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
		// TODO: add observer
	}

	conf := testnet.NewNetworkConfig("execution state sync test", net)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)
}

// TestHappyPath tests that Execution Nodes generate execution data, and Access Nodes are able to
// successfully sync the data
func (s *ExecutionStateSyncSuite) TestHappyPath() {
	// Let the network run for this many blocks
	runBlocks := uint64(20)

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

	// start an execution data service using the Access Node's execution data db
	an := s.net.ContainerByID(s.bridgeID)
	eds := s.nodeExecutionDataStore(an)

	// setup storage objects needed to get the execution data id
	db, err := an.DB()
	require.NoError(s.T(), err, "could not open db")

	metrics := metrics.NewNoopCollector()
	headers := storage.NewHeaders(metrics, db)
	results := storage.NewExecutionResults(metrics, db)

	// Loop through checkBlocks and verify the execution data was downloaded correctly
	for i := blockA.Header.Height; i <= blockA.Header.Height+checkBlocks; i++ {
		header, err := headers.ByHeight(i)
		require.NoError(s.T(), err, "could not get header")

		result, err := results.ByBlockID(header.ID())
		require.NoError(s.T(), err, "could not get sealed result")

		s.T().Logf("getting execution data for height %d, block %s, execution_data %s", header.Height, header.ID(), result.ExecutionDataID)

		ed, err := eds.Get(s.ctx, result.ExecutionDataID)
		if assert.NoError(s.T(), err, "could not get execution data for height %v", i) {
			s.T().Logf("got execution data for height %d", i)
			assert.Equal(s.T(), header.ID(), ed.BlockID)
		}
	}
}

func (s *ExecutionStateSyncSuite) nodeExecutionDataStore(node *testnet.Container) execution_data.ExecutionDataStore {
	ds, err := badgerds.NewDatastore(filepath.Join(node.ExecutionDataDBPath(), "blobstore"), &badgerds.DefaultOptions)
	require.NoError(s.T(), err, "could not get execution datastore")

	go func() {
		<-s.ctx.Done()
		if err := ds.Close(); err != nil {
			s.T().Logf("could not close execution data datastore: %v", err)
		}
	}()

	return execution_data.NewExecutionDataStore(blobs.NewBlobstore(ds), execution_data.DefaultSerializer)
}
