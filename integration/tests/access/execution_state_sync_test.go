package access

import (
	"context"
	"fmt"
	"os"
	"testing"

	badgerds "github.com/ipfs/go-ds-badger2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/network/compressor"
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
	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("testfile", "unstaked.go").
		Str("testcase", s.T().Name()).
		Logger()

	s.log = logger
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
	ghost := s.net.ContainerByID(s.ghostID)
	client, err := lib.GetGhostClient(ghost)
	require.NoError(s.T(), err, "could not get ghost client")
	return client
}

func (s *ExecutionStateSyncSuite) buildNetworkConfig() {
	// staked access node
	s.bridgeID = unittest.IdentifierFixture()
	bridgeANConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithID(s.bridgeID),
		testnet.SupportsUnstakedNodes(),
		testnet.WithLogLevel(zerolog.DebugLevel),
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
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
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

	// consensus followers
	conf := testnet.NewNetworkConfig("execution state sync test", net)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf)
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
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T())
	s.T().Logf("got block height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// wait for the requested number of sealed blocks, then pause the network so we can inspect the dbs
	s.BlockState.WaitForSealed(s.T(), blockA.Header.Height+runBlocks)
	s.net.StopContainers()

	// start an execution data service using the Access Node's execution data db
	an := s.net.ContainerByID(s.bridgeID)
	eds, ctx := s.nodeExecutionDataService(an)

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

		ed, err := eds.Get(ctx, result.ExecutionDataID)
		assert.NoError(s.T(), err, "could not get execution data for height %v", i)

		s.T().Logf("got execution data for height %d", i)
		assert.Equal(s.T(), header.ID(), ed.BlockID)
	}
}

func (s *ExecutionStateSyncSuite) nodeExecutionDataService(node *testnet.Container) (state_synchronization.ExecutionDataService, irrecoverable.SignalerContext) {
	ctx, errChan := irrecoverable.WithSignaler(s.ctx)
	go func() {
		select {
		case <-s.ctx.Done():
			return
		case err := <-errChan:
			s.T().Errorf("irrecoverable error: %v", err)
		}
	}()

	ds, err := badgerds.NewDatastore(node.ExecutionDataDBPath(), &badgerds.DefaultOptions)
	require.NoError(s.T(), err, "could not get execution datastore")

	go func() {
		<-s.ctx.Done()
		if err := ds.Close(); err != nil {
			s.T().Logf("could not close execution data datastore: %v", err)
		}
	}()

	blobService := utils.NewLocalBlobService(ds, utils.WithHashOnRead(true))
	blobService.Start(ctx)

	return state_synchronization.NewExecutionDataService(
		new(cbor.Codec),
		compressor.NewLz4Compressor(),
		blobService,
		metrics.NewNoopCollector(),
		zerolog.New(os.Stdout).With().Str("test", "execution-state-sync").Logger(),
	), ctx
}
