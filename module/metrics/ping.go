package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/flow"
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
		}, []string{LabelNodeID}),
	}
	return pc
}

func (pc *PingCollector) NodeReachable(nodeID flow.Identifier, reachable bool) {
	var val float64
	if reachable {
		val = 1
	}
	pc.reachable.With(prometheus.Labels{LabelNodeID: nodeID.String()}).Set(val)
}
