package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// NodeInfoCollector implements metrics to report static information about node.
// Such information can include: version, commit, sporkID.
type NodeInfoCollector struct {
	nodeInfo *prometheus.GaugeVec
}

const (
	sporkIDLabel              = "spork_id"
	versionLabel              = "version"
	commitLabel               = "commit"
	protocolStateVersionLabel = "protocol_state_version"
)

func NewNodeInfoCollector() *NodeInfoCollector {
	collector := &NodeInfoCollector{
		nodeInfo: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "node_info",
			Namespace: "general",
			Help:      "report general information about node, such information can include version, sporkID, commit hash, etc",
		}, []string{"key", "value"}),
	}

	return collector
}

func (sc *NodeInfoCollector) NodeInfo(version, commit, sporkID string, protocolStateVersion uint64) {
	sc.nodeInfo.WithLabelValues(versionLabel, version)
	sc.nodeInfo.WithLabelValues(commitLabel, commit)
	sc.nodeInfo.WithLabelValues(sporkIDLabel, sporkID)
	sc.nodeInfo.WithLabelValues(protocolStateVersionLabel, strconv.FormatUint(protocolStateVersion, 10))
}
