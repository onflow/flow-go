package rpc_inspector

import (
	"context"
	"fmt"
	"time"

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
	for i := range s.NodeConfigs {
		s.NodeConfigs[i].EnableMetricsServer = true
	}

	name := "bft_control_message_validation_false_positive_test"
	// short epoch lens ensure faster state commitments
	stakingAuctionLen := uint64(10)
	dkgPhaseLen := uint64(50)
	epochLen := uint64(300)
	epochCommitSafetyThreshold := uint64(50)
	netConfig := testnet.NewNetworkConfigWithEpochConfig(name, s.NodeConfigs, stakingAuctionLen, dkgPhaseLen, epochLen, epochCommitSafetyThreshold)
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
	_, err := utils.CreateFlowAccount(ctx, s.client)
	require.NoError(s.T(), err)
}

// ensureNoNotificationsDisseminated ensures the metrics result for the rpc inspector notification queue cache size metric for each container is 0
// indicating no notifications have been disseminated.
func (s *Suite) ensureNoNotificationsDisseminated(metricEndpoints map[string][]*io_prometheus_client.Metric) {
	for containerName, metric := range metricEndpoints {
		val := metric[0].GetGauge().GetValue()
		require.Zerof(s.T(), val, fmt.Sprintf("expected inspector notification queue cache size for container %s to be 0 got %v", containerName, val))
	}
}

// inspectorNotifQSizeMetricName returns the metric name for the rpc inspector notification queue cache size.
func (s *Suite) inspectorNotificationQSizeMetricName() string {
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

// loaderLoop submits load to the network in the form of account creation on the provided interval simulating some network traffic.
func (s *Suite) loaderLoop(ctx context.Context, numOfTestAccounts int, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for i := 0; i < numOfTestAccounts; i++ {
					s.submitSmokeTestTransaction(s.Ctx)
				}
			}
		}
	}()
}

// waitStateCommitments waits for n number of state commitment changes.
func (s *Suite) waitForStateCommitments(ctx context.Context, n int, waitFor, tick time.Duration) {
	prevStateComm := s.getCurrentFinalExecutionStateCommitment(ctx)
	numOfStateCommChanges := 0
	require.Eventually(s.T(), func() bool {
		currStateComm := s.getCurrentFinalExecutionStateCommitment(ctx)
		if prevStateComm != currStateComm {
			numOfStateCommChanges++
			prevStateComm = currStateComm
		}
		return numOfStateCommChanges >= n
	}, waitFor, tick)
}

// getCurrentFinalizedHeight returns the current finalized height.
func (s *Suite) getCurrentFinalExecutionStateCommitment(ctx context.Context) string {
	snapshot, err := s.client.GetLatestProtocolSnapshot(ctx)
	require.NoError(s.T(), err)
	executionResult, _, err := snapshot.SealedResult()
	require.NoError(s.T(), err)
	sc, err := executionResult.FinalStateCommitment()
	require.NoError(s.T(), err)
	bz, err := sc.MarshalJSON()
	require.NoError(s.T(), err)
	return string(bz)
}
