package metrics

import (
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNetworkCollector_OnClusterTopicMetricsCleanup verifies that OnClusterTopicMetricsCleanup
// properly cleans up all metrics associated with a cluster topic, including the top-offender
// metrics identified in the issue: inboundMessageSize, outboundMessageSize, and receivedIHaveMsgIDsHistogram.
func TestNetworkCollector_OnClusterTopicMetricsCleanup(t *testing.T) {
	// Use a unique prefix to avoid metric registration conflicts between tests
	prefix := fmt.Sprintf("test_%s_", t.Name())
	logger := zerolog.Nop()
	nc := NewNetworkCollector(logger, WithNetworkPrefix(prefix))

	clusterTopic := "sync-cluster-test-cluster-id"
	otherTopic := "blocks"
	protocol := "pubsub"
	msgType := "BlockProposal"

	// Record metrics for both cluster and non-cluster topics
	// This simulates normal operation where metrics accumulate for various topics
	nc.InboundMessageReceived(100, clusterTopic, protocol, msgType)
	nc.InboundMessageReceived(200, clusterTopic, protocol, "Transaction")
	nc.InboundMessageReceived(300, otherTopic, protocol, msgType)

	nc.OutboundMessageSent(150, clusterTopic, protocol, msgType)
	nc.OutboundMessageSent(250, otherTopic, protocol, msgType)

	nc.DuplicateInboundMessagesDropped(clusterTopic, protocol, msgType)
	nc.DuplicateInboundMessagesDropped(otherTopic, protocol, msgType)

	// Record LocalGossipSubRouterMetrics
	nc.OnLocalMeshSizeUpdated(clusterTopic, 5)
	nc.OnLocalMeshSizeUpdated(otherTopic, 3)
	nc.OnPeerGraftTopic(clusterTopic)
	nc.OnPeerGraftTopic(otherTopic)
	nc.OnPeerPruneTopic(clusterTopic)
	nc.OnPeerPruneTopic(otherTopic)

	// Record GossipSubRpcValidationInspectorMetrics
	nc.OnIHaveMessageIDsReceived(clusterTopic, 10)
	nc.OnIHaveMessageIDsReceived(otherTopic, 5)

	// Verify metrics exist before cleanup
	assertHistogramVecHasLabel(t, nc.inboundMessageSize, LabelChannel, clusterTopic, "inboundMessageSize should have cluster topic before cleanup")
	assertHistogramVecHasLabel(t, nc.inboundMessageSize, LabelChannel, otherTopic, "inboundMessageSize should have other topic before cleanup")
	assertHistogramVecHasLabel(t, nc.outboundMessageSize, LabelChannel, clusterTopic, "outboundMessageSize should have cluster topic before cleanup")
	assertHistogramVecHasLabel(t, nc.outboundMessageSize, LabelChannel, otherTopic, "outboundMessageSize should have other topic before cleanup")
	assertCounterVecHasLabel(t, nc.duplicateMessagesDropped, LabelChannel, clusterTopic, "duplicateMessagesDropped should have cluster topic before cleanup")
	assertCounterVecHasLabel(t, nc.duplicateMessagesDropped, LabelChannel, otherTopic, "duplicateMessagesDropped should have other topic before cleanup")

	// Perform cleanup for the cluster topic
	nc.OnClusterTopicMetricsCleanup(clusterTopic)

	// Verify cluster topic metrics are cleaned up
	assertHistogramVecNotHasLabel(t, nc.inboundMessageSize, LabelChannel, clusterTopic, "inboundMessageSize should NOT have cluster topic after cleanup")
	assertHistogramVecNotHasLabel(t, nc.outboundMessageSize, LabelChannel, clusterTopic, "outboundMessageSize should NOT have cluster topic after cleanup")
	assertCounterVecNotHasLabel(t, nc.duplicateMessagesDropped, LabelChannel, clusterTopic, "duplicateMessagesDropped should NOT have cluster topic after cleanup")

	// Verify other topic metrics are NOT cleaned up
	assertHistogramVecHasLabel(t, nc.inboundMessageSize, LabelChannel, otherTopic, "inboundMessageSize should still have other topic after cleanup")
	assertHistogramVecHasLabel(t, nc.outboundMessageSize, LabelChannel, otherTopic, "outboundMessageSize should still have other topic after cleanup")
	assertCounterVecHasLabel(t, nc.duplicateMessagesDropped, LabelChannel, otherTopic, "duplicateMessagesDropped should still have other topic after cleanup")
}

// TestNetworkCollector_OnClusterTopicMetricsCleanup_Idempotent verifies that calling
// OnClusterTopicMetricsCleanup multiple times for the same topic doesn't cause issues.
func TestNetworkCollector_OnClusterTopicMetricsCleanup_Idempotent(t *testing.T) {
	prefix := fmt.Sprintf("test_%s_", t.Name())
	logger := zerolog.Nop()
	nc := NewNetworkCollector(logger, WithNetworkPrefix(prefix))

	clusterTopic := "sync-cluster-test-cluster-id"
	protocol := "pubsub"
	msgType := "BlockProposal"

	// Record some metrics
	nc.InboundMessageReceived(100, clusterTopic, protocol, msgType)
	nc.OutboundMessageSent(150, clusterTopic, protocol, msgType)
	nc.OnIHaveMessageIDsReceived(clusterTopic, 10)

	// Call cleanup multiple times - should not panic or cause issues
	nc.OnClusterTopicMetricsCleanup(clusterTopic)
	nc.OnClusterTopicMetricsCleanup(clusterTopic)
	nc.OnClusterTopicMetricsCleanup(clusterTopic)

	// Verify metrics are cleaned up
	assertHistogramVecNotHasLabel(t, nc.inboundMessageSize, LabelChannel, clusterTopic, "inboundMessageSize should be cleaned up")
	assertHistogramVecNotHasLabel(t, nc.outboundMessageSize, LabelChannel, clusterTopic, "outboundMessageSize should be cleaned up")
}

// TestNetworkCollector_OnClusterTopicMetricsCleanup_NonexistentTopic verifies that calling
// OnClusterTopicMetricsCleanup for a topic that was never recorded doesn't cause issues.
func TestNetworkCollector_OnClusterTopicMetricsCleanup_NonexistentTopic(t *testing.T) {
	prefix := fmt.Sprintf("test_%s_", t.Name())
	logger := zerolog.Nop()
	nc := NewNetworkCollector(logger, WithNetworkPrefix(prefix))

	// Call cleanup for a topic that was never used - should not panic
	nc.OnClusterTopicMetricsCleanup("nonexistent-cluster-topic")
}

// assertHistogramVecHasLabel checks that the histogram vec has at least one metric with the given label value.
func assertHistogramVecHasLabel(t *testing.T, hv *prometheus.HistogramVec, labelName, labelValue, msg string) {
	t.Helper()
	found := histogramVecHasLabelValue(t, hv, labelName, labelValue)
	assert.True(t, found, msg)
}

// assertHistogramVecNotHasLabel checks that the histogram vec has no metrics with the given label value.
func assertHistogramVecNotHasLabel(t *testing.T, hv *prometheus.HistogramVec, labelName, labelValue, msg string) {
	t.Helper()
	found := histogramVecHasLabelValue(t, hv, labelName, labelValue)
	assert.False(t, found, msg)
}

// assertCounterVecHasLabel checks that the counter vec has at least one metric with the given label value.
func assertCounterVecHasLabel(t *testing.T, cv *prometheus.CounterVec, labelName, labelValue, msg string) {
	t.Helper()
	found := counterVecHasLabelValue(t, cv, labelName, labelValue)
	assert.True(t, found, msg)
}

// assertCounterVecNotHasLabel checks that the counter vec has no metrics with the given label value.
func assertCounterVecNotHasLabel(t *testing.T, cv *prometheus.CounterVec, labelName, labelValue, msg string) {
	t.Helper()
	found := counterVecHasLabelValue(t, cv, labelName, labelValue)
	assert.False(t, found, msg)
}

// histogramVecHasLabelValue collects metrics from the histogram vec and checks if any have the given label value.
func histogramVecHasLabelValue(t *testing.T, hv *prometheus.HistogramVec, labelName, labelValue string) bool {
	t.Helper()
	ch := make(chan prometheus.Metric, 100)
	go func() {
		hv.Collect(ch)
		close(ch)
	}()

	for metric := range ch {
		var m io_prometheus_client.Metric
		err := metric.Write(&m)
		require.NoError(t, err)

		for _, label := range m.GetLabel() {
			if label.GetName() == labelName && label.GetValue() == labelValue {
				return true
			}
		}
	}
	return false
}

// counterVecHasLabelValue collects metrics from the counter vec and checks if any have the given label value.
func counterVecHasLabelValue(t *testing.T, cv *prometheus.CounterVec, labelName, labelValue string) bool {
	t.Helper()
	ch := make(chan prometheus.Metric, 100)
	go func() {
		cv.Collect(ch)
		close(ch)
	}()

	for metric := range ch {
		var m io_prometheus_client.Metric
		err := metric.Write(&m)
		require.NoError(t, err)

		for _, label := range m.GetLabel() {
			if label.GetName() == labelName && label.GetValue() == labelValue {
				return true
			}
		}
	}
	return false
}
