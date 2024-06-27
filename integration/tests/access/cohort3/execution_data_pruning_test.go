package cohort3

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	badgerds "github.com/ipfs/go-ds-badger2"
	"github.com/rs/zerolog"
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

func TestExecutionDataPruning(t *testing.T) {
	suite.Run(t, new(ExecutionDataPruningSuite))
}

type ExecutionDataPruningSuite struct {
	suite.Suite
	lib.TestnetStateTracker

	log zerolog.Logger

	ghostID      flow.Identifier
	observerName string
	// threshold defines the maximum height range and how frequently pruning is performed.
	threshold uint64

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

func (s *ExecutionDataPruningSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *ExecutionDataPruningSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	// access node
	accessNodeConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag("--supports-observer=true"),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
		testnet.WithAdditionalFlag("--execution-data-retry-delay=1s"),
		testnet.WithAdditionalFlag("--execution-data-indexing-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
		testnet.WithAdditionalFlag("--event-query-mode=execution-nodes-only"),
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
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=400ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.FatalLevel),
	}

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
		accessNodeConfig, // access_1
		ghostNode,        // access ghost
	}

	// add the observer node config
	s.observerName = testnet.PrimaryON
	s.threshold = 50

	observers := []testnet.ObserverConfig{{
		ContainerName: s.observerName,
		LogLevel:      zerolog.WarnLevel,
		AdditionalFlags: []string{
			fmt.Sprintf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
			fmt.Sprintf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
			"--execution-data-sync-enabled=true",
			"--execution-data-indexing-enabled=true",
			"--execution-data-retry-delay=1s",
			"--event-query-mode=local-only",
			fmt.Sprintf("--execution-data-height-range-target=%d", 100),            //400
			fmt.Sprintf("--execution-data-height-range-threshold=%d", s.threshold), //100
		},
	},
		{
			ContainerName: "observer_2",
			LogLevel:      zerolog.InfoLevel,
		},
	}

	conf := testnet.NewNetworkConfig("execution_data_pruning", nodeConfigs, testnet.WithObservers(observers...))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)
	s.Track(s.T(), s.ctx, s.Ghost())
}

func (s *ExecutionDataPruningSuite) Ghost() *client.GhostClient {
	client, err := s.net.ContainerByID(s.ghostID).GhostClient()
	require.NoError(s.T(), err, "could not get ghost client")
	return client
}

func (s *ExecutionDataPruningSuite) TestHappyPath() {
	// pause until the observer node is progressing
	//TODO: UlianaAndrukhiv: remove Sleep
	time.Sleep(2 * time.Minute)

	s.net.StopContainers()

	metrics := metrics.NewNoopCollector()

	an := s.net.ContainerByName(testnet.PrimaryAN)
	// setup storage objects needed to get the execution data id
	anDB, err := an.DB()
	require.NoError(s.T(), err, "could not open db")

	anResults := storage.NewExecutionResults(metrics, anDB)

	on := s.net.ContainerByName(s.observerName)
	onEds := s.nodeExecutionDataStore(on)
	// setup storage objects needed to get the execution data id
	onDB, err := on.DB()
	require.NoError(s.T(), err, "could not open db")

	onHeaders := storage.NewHeaders(metrics, onDB)
	onResults := storage.NewExecutionResults(metrics, onDB)

	// Loop through blocks and verify the execution data was pruned correctly
	startBlockHeight := uint64(1)

	for i := startBlockHeight; i <= s.threshold+1; i++ {
		header, err := onHeaders.ByHeight(i)
		require.NoError(s.T(), err, "%s: could not get header", s.observerName)

		result, err := anResults.ByBlockID(header.ID())
		require.NoError(s.T(), err, "%s: could not get sealed result", testnet.PrimaryAN)

		result, err = onResults.ByID(result.ID())
		require.NoError(s.T(), err, "%s: could not get sealed result from ON`s storage", testnet.PrimaryON)

		_, err = onEds.Get(s.ctx, result.ExecutionDataID)
		require.Error(s.T(), err, "%s: could not prune execution data for height %v", s.observerName, i)
		require.ErrorContains(s.T(), err, "not found")
		s.T().Logf("%s: execution data for height was pruned %d, err: %v", s.observerName, i, err)
	}
}

func (s *ExecutionDataPruningSuite) nodeExecutionDataStore(node *testnet.Container) execution_data.ExecutionDataStore {
	ds, err := badgerds.NewDatastore(filepath.Join(node.ExecutionDataDBPath(), "blobstore"), &badgerds.DefaultOptions)
	require.NoError(s.T(), err, "could not get execution datastore")

	return execution_data.NewExecutionDataStore(blobs.NewBlobstore(ds), execution_data.DefaultSerializer)
}
