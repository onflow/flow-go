package cohort3

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	badgerds "github.com/ipfs/go-ds-badger2"
	pebbleds "github.com/ipfs/go-ds-pebble"
	sdk "github.com/onflow/flow-go-sdk"
	sdkclient "github.com/onflow/flow-go-sdk/access/grpc"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
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

	net                 *testnet.FlowNetwork
	executionDataDBMode execution_data.ExecutionDataDBMode
}

func (s *ExecutionStateSyncSuite) SetupTest() {
	s.setup(execution_data.ExecutionDataDBModeBadger)
}

func (s *ExecutionStateSyncSuite) setup(executionDataDBMode execution_data.ExecutionDataDBMode) {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.executionDataDBMode = executionDataDBMode
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
		testnet.WithAdditionalFlag(fmt.Sprintf("--execution-data-db=%s", s.executionDataDBMode.String())),
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
		LogLevel:      zerolog.InfoLevel,
		AdditionalFlags: []string{
			fmt.Sprintf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
			"--execution-data-sync-enabled=true",
			"--event-query-mode=execution-nodes-only",
			fmt.Sprintf("--execution-data-db=%s", s.executionDataDBMode.String()),
		},
	}}

	conf := testnet.NewNetworkConfig("execution state sync test", net, testnet.WithObservers(observers...))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)
}

// TestBadgerDBHappyPath tests that Execution Nodes generate execution data, and Access Nodes are able to
// successfully sync the data to badger DB
func (s *ExecutionStateSyncSuite) TestBadgerDBHappyPath() {
	s.executionStateSyncTest()
}

func (s *ExecutionStateSyncSuite) executionStateSyncTest() {
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

	// Loop through checkBlocks and verify the execution data was downloaded correctly
	an := s.net.ContainerByName(testnet.PrimaryAN)
	anClient, err := an.SDKClient()
	require.NoError(s.T(), err, "could not get access node testnet client")

	on := s.net.ContainerByName(s.observerName)
	onClient, err := on.SDKClient()
	require.NoError(s.T(), err, "could not get observer testnet client")

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	for i := blockA.Header.Height; i <= blockA.Header.Height+checkBlocks; i++ {
		anBED, err := s.executionDataForHeight(ctx, anClient, i)
		require.NoError(s.T(), err, "could not get execution data from AN for height %v", i)

		onBED, err := s.executionDataForHeight(ctx, onClient, i)
		require.NoError(s.T(), err, "could not get execution data from ON for height %v", i)

		assert.Equal(s.T(), anBED.BlockID, onBED.BlockID)
	}
}

// executionDataForHeight returns the execution data for the given height from the given node
// It retries the request until the data is available or the context is canceled
func (s *ExecutionStateSyncSuite) executionDataForHeight(ctx context.Context, nodeClient *sdkclient.Client, height uint64) (*execution_data.BlockExecutionData, error) {
	execDataClient := nodeClient.ExecutionDataRPCClient()

	var header *sdk.BlockHeader
	s.Require().NoError(retryNotFound(ctx, 200*time.Millisecond, func() error {
		var err error
		header, err = nodeClient.GetBlockHeaderByHeight(s.ctx, height)
		return err
	}), "could not get block header for block %d", height)

	var blockED *execution_data.BlockExecutionData
	s.Require().NoError(retryNotFound(ctx, 200*time.Millisecond, func() error {
		ed, err := execDataClient.GetExecutionDataByBlockID(s.ctx, &executiondata.GetExecutionDataByBlockIDRequest{
			BlockId:              header.ID[:],
			EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
		})
		if err != nil {
			return err
		}

		blockED, err = convert.MessageToBlockExecutionData(ed.GetBlockExecutionData(), flow.Localnet.Chain())
		s.Require().NoError(err, "could not convert execution data")

		return err
	}), "could not get execution data for block %d", height)

	return blockED, nil
}

// retryNotFound retries the given function until it returns an error that is not NotFound or the context is canceled
func retryNotFound(ctx context.Context, delay time.Duration, f func() error) error {
	for ctx.Err() == nil {
		err := f()
		if status.Code(err) == codes.NotFound {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		return err
	}
	return ctx.Err()
}

func (s *ExecutionStateSyncSuite) nodeExecutionDataStore(node *testnet.Container) execution_data.ExecutionDataStore {
	var ds datastore.Batching
	var err error
	dsPath := filepath.Join(node.ExecutionDataDBPath(), "blobstore")

	if s.executionDataDBMode == execution_data.ExecutionDataDBModePebble {
		ds, err = pebbleds.NewDatastore(dsPath, nil)
	} else {
		ds, err = badgerds.NewDatastore(dsPath, &badgerds.DefaultOptions)
	}
	require.NoError(s.T(), err, "could not get execution datastore")

	return execution_data.NewExecutionDataStore(blobs.NewBlobstore(ds), execution_data.DefaultSerializer)
}
