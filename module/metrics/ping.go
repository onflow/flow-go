package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/model/flow"
)

type PingCollector struct {
	reachable *prometheus.GaugeVec
}

func NewPingCollector() *PingCollector {
	pc := &PingCollector{
		reachable: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "node_reachable",
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Help:      "report whether a node is reachable",
		}, []string{LabelNodeID, LabelNodeAddress, LabelNodeRole, LabelNodeInfo}),
	}
	return pc
}

func (pc *PingCollector) NodeReachable(node *flow.Identity, nodeInfo string, rtt time.Duration) {
	var rttValue float64
	if rtt > 0 {
		rttValue = float64(rtt.Milliseconds())
	} else {
		rttValue = -1
	}

	pc.reachable.With(prometheus.Labels{
		LabelNodeID:      node.NodeID.String(),
		LabelNodeAddress: node.Address,
		LabelNodeRole:    node.Role.String(),
		LabelNodeInfo:    nodeInfo}).
		Set(rttValue)
}
