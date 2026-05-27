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

// TestNetworkCollector_ClusterTopicNormalization verifies that cluster topics are normalized
// to their prefix (e.g., "sync-cluster-*" -> "sync-cluster") to prevent unbounded metric
// cardinality growth during epoch transitions.
func TestNetworkCollector_ClusterTopicNormalization(t *testing.T) {
	prefix := fmt.Sprintf("test_%s_", t.Name())
	logger := zerolog.Nop()
	nc := NewNetworkCollector(logger, WithNetworkPrefix(prefix))

	// These cluster topics should be normalized to their prefix
	clusterTopic1 := "sync-cluster-test-cluster-id-1"
	clusterTopic2 := "sync-cluster-test-cluster-id-2"
	consensusClusterTopic := "consensus-cluster-test-cluster-id"
	nonClusterTopic := "blocks"
	protocol := "pubsub"
	msgType := "BlockProposal"

	// Record metrics for multiple cluster topics and a non-cluster topic
	nc.InboundMessageReceived(100, clusterTopic1, protocol, msgType)
	nc.InboundMessageReceived(200, clusterTopic2, protocol, msgType)
	nc.InboundMessageReceived(300, consensusClusterTopic, protocol, msgType)
	nc.InboundMessageReceived(400, nonClusterTopic, protocol, msgType)

	nc.OutboundMessageSent(150, clusterTopic1, protocol, msgType)
	nc.OutboundMessageSent(250, clusterTopic2, protocol, msgType)
	nc.OutboundMessageSent(350, consensusClusterTopic, protocol, msgType)
	nc.OutboundMessageSent(450, nonClusterTopic, protocol, msgType)

	nc.DuplicateInboundMessagesDropped(clusterTopic1, protocol, msgType)
	nc.DuplicateInboundMessagesDropped(clusterTopic2, protocol, msgType)
	nc.DuplicateInboundMessagesDropped(consensusClusterTopic, protocol, msgType)
	nc.DuplicateInboundMessagesDropped(nonClusterTopic, protocol, msgType)

	// Verify cluster topics are normalized to their prefix
	// Both sync-cluster topics should be recorded under "sync-cluster"
	assertHistogramVecHasLabel(t, nc.inboundMessageSize, LabelChannel, "sync-cluster", "sync-cluster topics should be normalized to 'sync-cluster'")
	assertHistogramVecHasLabel(t, nc.inboundMessageSize, LabelChannel, "consensus-cluster", "consensus-cluster topics should be normalized to 'consensus-cluster'")

	// Verify the original cluster topic names are NOT present (they should be normalized)
	assertHistogramVecNotHasLabel(t, nc.inboundMessageSize, LabelChannel, clusterTopic1, "original cluster topic name should not be present")
	assertHistogramVecNotHasLabel(t, nc.inboundMessageSize, LabelChannel, clusterTopic2, "original cluster topic name should not be present")
	assertHistogramVecNotHasLabel(t, nc.inboundMessageSize, LabelChannel, consensusClusterTopic, "original consensus-cluster topic name should not be present")

	// Verify non-cluster topics are NOT normalized (they should keep their original name)
	assertHistogramVecHasLabel(t, nc.inboundMessageSize, LabelChannel, nonClusterTopic, "non-cluster topics should keep their original name")
}

// TestLocalGossipSubRouterMetrics_ClusterTopicNormalization verifies that LocalGossipSubRouterMetrics
// normalizes cluster topics to their prefix.
func TestLocalGossipSubRouterMetrics_ClusterTopicNormalization(t *testing.T) {
	prefix := fmt.Sprintf("test_%s_", t.Name())
	m := NewGossipSubLocalMeshMetrics(prefix)

	clusterTopic1 := "sync-cluster-cluster-1"
	clusterTopic2 := "sync-cluster-cluster-2"
	consensusClusterTopic := "consensus-cluster-cluster-1"
	nonClusterTopic := "blocks"

	// Record metrics for cluster and non-cluster topics
	m.OnLocalMeshSizeUpdated(clusterTopic1, 5)
	m.OnLocalMeshSizeUpdated(clusterTopic2, 3)
	m.OnLocalMeshSizeUpdated(consensusClusterTopic, 4)
	m.OnLocalMeshSizeUpdated(nonClusterTopic, 2)

	m.OnPeerGraftTopic(clusterTopic1)
	m.OnPeerGraftTopic(clusterTopic2)
	m.OnPeerGraftTopic(consensusClusterTopic)
	m.OnPeerGraftTopic(nonClusterTopic)

	m.OnPeerPruneTopic(clusterTopic1)
	m.OnPeerPruneTopic(clusterTopic2)
	m.OnPeerPruneTopic(consensusClusterTopic)
	m.OnPeerPruneTopic(nonClusterTopic)

	// Verify cluster topics are normalized to their prefix
	assertGaugeVecHasLabel(t, &m.localMeshSize, LabelChannel, "sync-cluster", "sync-cluster topics should be normalized")
	assertGaugeVecHasLabel(t, &m.localMeshSize, LabelChannel, "consensus-cluster", "consensus-cluster topics should be normalized")

	// Verify original cluster topic names are NOT present
	assertGaugeVecNotHasLabel(t, &m.localMeshSize, LabelChannel, clusterTopic1, "original cluster topic should not be present")
	assertGaugeVecNotHasLabel(t, &m.localMeshSize, LabelChannel, clusterTopic2, "original cluster topic should not be present")
	assertGaugeVecNotHasLabel(t, &m.localMeshSize, LabelChannel, consensusClusterTopic, "original consensus-cluster topic should not be present")

	// Verify non-cluster topics are NOT normalized
	assertGaugeVecHasLabel(t, &m.localMeshSize, LabelChannel, nonClusterTopic, "non-cluster topics should keep their original name")
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

// assertGaugeVecHasLabel checks that the gauge vec has at least one metric with the given label value.
func assertGaugeVecHasLabel(t *testing.T, gv *prometheus.GaugeVec, labelName, labelValue, msg string) {
	t.Helper()
	found := gaugeVecHasLabelValue(t, gv, labelName, labelValue)
	assert.True(t, found, msg)
}

// assertGaugeVecNotHasLabel checks that the gauge vec has no metrics with the given label value.
func assertGaugeVecNotHasLabel(t *testing.T, gv *prometheus.GaugeVec, labelName, labelValue, msg string) {
	t.Helper()
	found := gaugeVecHasLabelValue(t, gv, labelName, labelValue)
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

// gaugeVecHasLabelValue collects metrics from the gauge vec and checks if any have the given label value.
func gaugeVecHasLabelValue(t *testing.T, gv *prometheus.GaugeVec, labelName, labelValue string) bool {
	t.Helper()
	ch := make(chan prometheus.Metric, 100)
	go func() {
		gv.Collect(ch)
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
