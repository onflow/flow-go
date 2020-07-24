package metrics

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PingCollector struct {
	reachable *prometheus.GaugeVec
}

func NewPingCollector(interval time.Duration) *PingCollector {
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
