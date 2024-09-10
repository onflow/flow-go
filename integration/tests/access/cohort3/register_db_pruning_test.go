package cohort3

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	sdk "github.com/onflow/flow-go-sdk"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	pstorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
)

type RegisterDBPruningSuite struct {
	suite.Suite

	log zerolog.Logger

	accessNodeName   string
	observerNodeName string

	pebbleDb *pebble.DB

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

func TestRegisterDBPruning(t *testing.T) {
	suite.Run(t, new(RegisterDBPruningSuite))
}

func (s *RegisterDBPruningSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *RegisterDBPruningSuite) SetupTest() {
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
		testnet.WithAdditionalFlagf("--execution-state-dir=%s", s.getPebbleDBPath(s.accessNodeName)),
		testnet.WithAdditionalFlagf("--public-network-execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlagf("--event-query-mode=local-only"),
		testnet.WithAdditionalFlag("--registerdb-pruning-enabled=true"),
		testnet.WithAdditionalFlag("--registerdb-prune-throttle-delay=1s"),
		testnet.WithAdditionalFlag("--registerdb-prune-ticker-interval=10s"),
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

	observers := []testnet.ObserverConfig{{
		ContainerName: s.observerNodeName,
		LogLevel:      zerolog.InfoLevel,
		AdditionalFlags: []string{
			fmt.Sprintf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
			fmt.Sprintf("--execution-state-dir=%s", s.getPebbleDBPath(s.observerNodeName)),
			"--execution-data-sync-enabled=true",
			"--execution-data-indexing-enabled=true",
			"--execution-data-retry-delay=1s",
			"--event-query-mode=local-only",
			"--local-service-api-enabled=true",
			"--registerdb-pruning-enabled=true",
			"--registerdb-prune-throttle-delay=1s",
			"--registerdb-prune-ticker-interval=10s",
		},
	}}

	conf := testnet.NewNetworkConfig("register_db_pruning", nodeConfigs, testnet.WithObservers(observers...))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)
}

func (s *RegisterDBPruningSuite) TestHappyPath() {
	//accessNode := s.net.ContainerByName(s.accessNodeName)
	observerNode := s.net.ContainerByName(s.observerNodeName)

	waitingBlockHeight := uint64(200)
	s.waitUntilExecutionDataForBlockIndexed(observerNode, waitingBlockHeight)
	s.net.StopContainers()

	pebbleAN := s.getPebbleDB(s.accessNodeName)
	registerAN := s.nodeRegisterStorage(pebbleAN)

	pebbleON := s.getPebbleDB(s.observerNodeName)
	registerON := s.nodeRegisterStorage(pebbleON)

	assert.Equal(s.T(), registerAN.LatestHeight(), registerON.LatestHeight())
}

func (s *RegisterDBPruningSuite) getPebbleDBPath(containerName string) string {
	return filepath.Join(testnet.DefaultExecutionStateDir, containerName)
}

func (s *RegisterDBPruningSuite) getPebbleDB(containerName string) *pebble.DB {
	if s.pebbleDb == nil {
		var err error
		s.pebbleDb, err = pstorage.OpenRegisterPebbleDB(s.getPebbleDBPath(containerName))
		require.NoError(s.T(), err, "could not open db")
	}
	return s.pebbleDb
}

func (s *RegisterDBPruningSuite) nodeRegisterStorage(db *pebble.DB) *pstorage.Registers {
	registers, err := pstorage.NewRegisters(db)
	s.Require().NoError(err)
	return registers
}

// getGRPCClient is the helper func to create an access api client
func (s *RegisterDBPruningSuite) getGRPCClient(address string) (accessproto.AccessAPIClient, error) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := accessproto.NewAccessAPIClient(conn)
	return client, nil
}

// waitUntilExecutionDataForBlockIndexed waits until the execution data for the specified block height is indexed.
// It subscribes to events from the start height and waits until the execution data for the specified block height is indexed.
func (s *RegisterDBPruningSuite) waitUntilExecutionDataForBlockIndexed(observerNode *testnet.Container, waitingBlockHeight uint64) {
	grpcClient, err := s.getGRPCClient(observerNode.Addr(testnet.GRPCPort))
	s.Require().NoError(err)

	// creating execution data api client
	client, err := getClient(fmt.Sprintf("localhost:%s", observerNode.Port(testnet.ExecutionStatePort)))
	s.Require().NoError(err)

	// pause until the observer node start indexing blocks,
	// getting events from 1-nd block to make sure that 1-st block already indexed, and we can start subscribing
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

	duration := 3 * time.Minute
	for {
		select {
		case err := <-errChan:
			s.Require().NoErrorf(err, "unexpected %s error", s.observerNodeName)
		case event := <-eventsChan:
			if event.Height >= waitingBlockHeight {
				return
			}
		case <-time.After(duration):
			s.T().Fatalf("failed to index to %d block within %s", waitingBlockHeight, duration.String())
		}
	}
}
