package cohort2

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

// This suite tests collection syncing using the ingestion engine and the indexer.

const lastFullBlockMetric = "access_ingestion_last_full_finalized_block_height"

func TestCollectionIndexing(t *testing.T) {
	suite.Run(t, new(CollectionIndexingSuite))
}

type CollectionIndexingSuite struct {
	suite.Suite
	net *testnet.FlowNetwork

	cancel context.CancelFunc
}

func (s *CollectionIndexingSuite) SetupTest() {
	// access_1 is not running the indexer, so all collections are indexed using the ingestion engine
	defaultAccessOpts := []func(config *testnet.NodeConfig){
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
		testnet.WithAdditionalFlagf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
		testnet.WithMetricsServer(),
	}
	// access_2 is running the indexer, so all collections are indexed using the indexer
	testANOpts := append(defaultAccessOpts,
		testnet.WithLogLevel(zerolog.DebugLevel),
		testnet.WithAdditionalFlag("--execution-data-indexing-enabled=true"),
	)

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleAccess, defaultAccessOpts...),
		testnet.NewNodeConfig(flow.RoleAccess, testANOpts...),
	}

	// prepare the network
	conf := testnet.NewNetworkConfig("access_collection_indexing_test", nodeConfigs)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	s.net.Start(ctx)
}

func (s *CollectionIndexingSuite) TearDownTest() {
	if s.net != nil {
		s.net.Remove()
		s.net = nil
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (s *CollectionIndexingSuite) Test() {
	// start the network with access_2 paused.
	// this simulates it falling behind on syncing collections
	access2 := s.net.ContainerByName("access_2")
	s.Require().NoError(access2.Disconnect())

	// wait for access_1 to sync collections
	targetBlockCount := uint64(50)
	s.Eventually(func() bool {
		value, err := s.getLastFullHeight("access_1")
		s.T().Logf("access_1 last full height: %d", value)
		return err == nil && value > targetBlockCount
	}, 60*time.Second, 1*time.Second)

	// stop the collection nodes
	// this will prevent access_2 from syncing collections from the network
	s.Require().NoError(s.net.ContainerByName("collection_1").Disconnect())
	s.Require().NoError(s.net.ContainerByName("collection_2").Disconnect())

	// now start access_2, and wait for it to catch up with collections
	s.Require().NoError(access2.Connect())

	s.Eventually(func() bool {
		value, err := s.getLastFullHeight("access_2")
		s.T().Logf("access_2 last full height: %d", value)
		return err == nil && value > targetBlockCount
	}, 60*time.Second, 1*time.Second)
}

func (s *CollectionIndexingSuite) getLastFullHeight(containerName string) (uint64, error) {
	node := s.net.ContainerByName(containerName)
	metricsURL := fmt.Sprintf("http://0.0.0.0:%s/metrics", node.Port(testnet.MetricsPort))
	values := s.net.GetMetricFromContainer(s.T(), containerName, metricsURL, lastFullBlockMetric)

	if len(values) == 0 {
		return 0, fmt.Errorf("no values found")
	}

	return uint64(values[0].GetGauge().GetValue()), nil
}
