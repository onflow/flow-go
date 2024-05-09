package metrics

import (
	"strconv"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestNodeInfoCollector_NodeInfo tests if node info collector reports desired metrics
func TestNodeInfoCollector_NodeInfo(t *testing.T) {
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	collector := NewNodeInfoCollector()
	version := "0.29"
	commit := "63cec231136914941e2358de2054a6ef71ea3c99"
	sporkID := unittest.IdentifierFixture().String()
	protocolVersion := uint(10076)
	collector.NodeInfo(version, commit, sporkID, protocolVersion)
	metricsFamilies, err := reg.Gather()
	require.NoError(t, err)

	assertReported := func(value string) {
		for _, metric := range metricsFamilies[0].Metric {
			for _, label := range metric.GetLabel() {
				if label.GetValue() == value {
					return
				}
			}
		}
		assert.Failf(t, "metric not found", "except to find value %s", value)
	}

	protocolVersionAsString := strconv.FormatUint(uint64(protocolVersion), 10)
	for _, value := range []string{version, commit, sporkID, protocolVersionAsString} {
		assertReported(value)
	}
}
