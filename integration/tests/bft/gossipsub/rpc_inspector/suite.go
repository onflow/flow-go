package rpc_inspector

import (
	"context"
	"fmt"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

// Suite represents a test suite that sets up a full flow network.
type Suite struct {
	bft.BaseSuite
	client *testnet.Client
}

// SetupSuite generates, initializes, and starts the Flow network.
func (s *Suite) SetupSuite() {
	s.BaseSuite.SetupSuite()

	// enable metrics server for all nodes
	for i, _ := range s.NodeConfigs {
		s.NodeConfigs[i].EnableMetricsServer = true
	}

	name := "bft_control_message_validation_false_positive_test"
	netConfig := testnet.NewNetworkConfig(
		name,
		s.NodeConfigs,
		// set long staking phase to avoid QC/DKG transactions during test run
		testnet.WithViewsInStakingAuction(10_000),
		testnet.WithViewsInEpoch(100_000),
	)

	s.Net = testnet.PrepareFlowNetwork(s.T(), netConfig, flow.BftTestnet)

	s.Ctx, s.Cancel = context.WithCancel(context.Background())
	s.Net.Start(s.Ctx)

	// starts tracking blocks by the ghost node
	s.Track(s.T(), s.Ctx, s.Ghost())

	client, err := s.Net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	require.NoError(s.T(), err)
	s.client = client
}

// submitSmokeTestTransaction will submit a create account transaction to smoke test network
// This ensures a single transaction can be sealed by the network.
func (s *Suite) submitSmokeTestTransaction(ctx context.Context) {
	addr, err := utils.CreateFlowAccount(ctx, s.client)
	require.NoError(s.T(), err)
}

// ensureNoNotificationsDisseminated ensures the metrics result for the rpc inspector notification queue cache size metric for each container is 0
// indicating no notifications have been disseminated.
func (s *Suite) ensureNoNotificationsDisseminated(mets map[string][]*io_prometheus_client.Metric) {
	for containerName, metric := range mets {
		val := metric[0].GetGauge().GetValue()
		require.Zerof(s.T(), val, fmt.Sprintf("expected inspector notification queue cache size for container %s to be 0 got %v", containerName, val))
	}
}

// inspectorNotifQSizeMetricName returns the metric name for the rpc inspector notification queue cache size.
func (s *Suite) inspectorNotifQSizeMetricName() string {
	return fmt.Sprintf("network_hero_cache_%s_successful_write_count_total", metrics.ResourceNetworkingRpcInspectorNotificationQueue)
}

// metricsUrls returns a list of metrics urls for each node configured on the test suite.
func (s *Suite) metricsUrls() map[string]string {
	urls := make(map[string]string, 0)
	for containerName, port := range s.Net.PortsByContainerName(testnet.MetricsPort, false) {
		urls[containerName] = fmt.Sprintf("http://0.0.0.0:%s/metrics", port)
	}
	return urls
}
