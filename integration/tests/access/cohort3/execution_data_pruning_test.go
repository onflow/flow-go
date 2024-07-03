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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/metrics"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"
)

func TestExecutionDataPruning(t *testing.T) {
	suite.Run(t, new(ExecutionDataPruningSuite))
}

type ExecutionDataPruningSuite struct {
	suite.Suite

	log zerolog.Logger

	accessNodeName   string
	observerNodeName string
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
	s.accessNodeName = testnet.PrimaryAN
	accessNodeConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag("--supports-observer=true"),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
		testnet.WithAdditionalFlag("--execution-data-retry-delay=1s"),
		testnet.WithAdditionalFlag("--execution-data-indexing-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
		testnet.WithAdditionalFlagf("--public-network-execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlagf("--event-query-mode=local-only"),
		testnet.WithAdditionalFlagf("--execution-data-height-range-target=%d", 100),
		testnet.WithAdditionalFlagf("--execution-data-height-range-threshold=%d", s.threshold),
	)

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
	}

	// add the observer node config
	s.observerNodeName = testnet.PrimaryON
	s.threshold = 50

	observers := []testnet.ObserverConfig{{
		ContainerName: s.observerNodeName,
		LogLevel:      zerolog.InfoLevel,
		AdditionalFlags: []string{
			fmt.Sprintf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
			fmt.Sprintf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
			"--execution-data-sync-enabled=true",
			"--execution-data-indexing-enabled=true",
			"--execution-data-retry-delay=1s",
			"--event-query-mode=local-only",
			fmt.Sprintf("--execution-data-height-range-target=%d", 100),
			fmt.Sprintf("--execution-data-height-range-threshold=%d", s.threshold),
		},
	}}

	conf := testnet.NewNetworkConfig("execution_data_pruning", nodeConfigs, testnet.WithObservers(observers...))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)
}

// getGRPCClient is the helper func to create an access api client
func (s *ExecutionDataPruningSuite) getGRPCClient(address string) (accessproto.AccessAPIClient, error) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := accessproto.NewAccessAPIClient(conn)
	return client, nil
}

func (s *ExecutionDataPruningSuite) TestHappyPath() {
	accessNode := s.net.ContainerByName(s.accessNodeName)
	observerNode := s.net.ContainerByName(s.observerNodeName)

	waitingBlockHeight := uint64(200)
	s.waitUntilExecutionDataForBlockIndexed(waitingBlockHeight)
	s.net.StopContainers()

	metrics := metrics.NewNoopCollector()

	// start an execution data service using the Access Node's execution data db
	anEds := s.nodeExecutionDataStore(accessNode)

	// setup storage objects needed to get the execution data id
	anDB, err := accessNode.DB()
	require.NoError(s.T(), err, "could not open db")

	anHeaders := storage.NewHeaders(metrics, anDB)
	anResults := storage.NewExecutionResults(metrics, anDB)

	// start an execution data service using the Observer Node's execution data db

	onEds := s.nodeExecutionDataStore(observerNode)
	// setup storage objects needed to get the execution data id
	onDB, err := observerNode.DB()
	require.NoError(s.T(), err, "could not open db")

	onResults := storage.NewExecutionResults(metrics, onDB)

	s.checkResults(anHeaders, anResults, onResults, anEds, onEds)
}

// waitUntilExecutionDataForBlockIndexed waits until the execution data for the specified block height is indexed.
func (s *ExecutionDataPruningSuite) waitUntilExecutionDataForBlockIndexed(waitingBlockHeight uint64) {
	observerNode := s.net.ContainerByName(s.observerNodeName)

	grpcClient, err := s.getGRPCClient(observerNode.Addr(testnet.GRPCPort))
	s.Require().NoError(err)

	// creating execution data api client
	client, err := getClient(fmt.Sprintf("localhost:%s", observerNode.Port(testnet.ExecutionStatePort)))
	s.Require().NoError(err)

	// pause until the observer node start indexing blocks
	s.Require().Eventually(func() bool {
		_, err := grpcClient.GetEventsForHeightRange(s.ctx, &accessproto.GetEventsForHeightRangeRequest{
			Type:                 sdk.EventAccountCreated,
			StartHeight:          1,
			EndHeight:            1,
			EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
		})

		return err == nil
	}, 2*time.Minute, 10*time.Second)

	// subscribe on events till waitingBlockHeight to make sure that execution data for block indexed till waitingBlockHeight and pruner
	// pruned execution data at least once
	// SubscribeEventsFromStartHeight used as subscription here because we need to make sure that execution data are already indexed
	stream, err := client.SubscribeEventsFromStartHeight(s.ctx, &executiondata.SubscribeEventsFromStartHeightRequest{
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
		Filter:               &executiondata.EventFilter{},
		HeartbeatInterval:    1,
		StartBlockHeight:     0,
	})
	s.Require().NoError(err)
	eventsChan, errChan, err := SubscribeHandler(s.ctx, stream.Recv, eventsResponseHandler)
	s.Require().NoError(err)

	for {
		select {
		case err := <-errChan:
			s.Require().NoErrorf(err, "unexpected %s error", s.observerNodeName)
		case event := <-eventsChan:
			if event.Height > waitingBlockHeight {
				return
			}
		}
	}
}

// checkResults checks the results of execution data pruning to ensure correctness.
func (s *ExecutionDataPruningSuite) checkResults(
	headers *storage.Headers,
	anResults *storage.ExecutionResults,
	onResults *storage.ExecutionResults,
	anEds execution_data.ExecutionDataStore,
	onEds execution_data.ExecutionDataStore,
) {
	// Loop through blocks and verify the execution data was pruned correctly
	// execution data till height equal threshold + 1 should be pruned

	// checking execution results from 1 block
	startBlockHeight := uint64(1)
	for i := startBlockHeight; i <= s.threshold+1; i++ {
		header, err := headers.ByHeight(i)
		require.NoError(s.T(), err, "%s: could not get header", s.accessNodeName)

		result, err := anResults.ByBlockID(header.ID())
		require.NoError(s.T(), err, "%s: could not get sealed result", s.accessNodeName)

		// verify AN execution data
		_, err = anEds.Get(s.ctx, result.ExecutionDataID)
		require.Error(s.T(), err, "%s: could not prune execution data for height %v", s.accessNodeName, i)
		require.ErrorContains(s.T(), err, "not found")

		result, err = onResults.ByID(result.ID())
		require.NoError(s.T(), err, "%s: could not get sealed result from ON`s storage", s.observerNodeName)

		// verify ON execution data
		_, err = onEds.Get(s.ctx, result.ExecutionDataID)
		require.Error(s.T(), err, "%s: could not prune execution data for height %v", s.observerNodeName, i)
		require.ErrorContains(s.T(), err, "not found")
	}
}

func (s *ExecutionDataPruningSuite) nodeExecutionDataStore(node *testnet.Container) execution_data.ExecutionDataStore {
	ds, err := badgerds.NewDatastore(filepath.Join(node.ExecutionDataDBPath(), "blobstore"), &badgerds.DefaultOptions)
	require.NoError(s.T(), err, "could not get execution datastore")

	return execution_data.NewExecutionDataStore(blobs.NewBlobstore(ds), execution_data.DefaultSerializer)
}
