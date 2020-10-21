package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type NodeController struct {
	restart prometheus.Counter
}

func NewNodeCollector() *NodeController {
	return &NodeController{
		restart: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "restarted",
			Namespace: namespaceNode,
			Help:      "report number of times a node was restarted",
		}),
	}
}

func (rc *NodeController) Restarted() {
	rc.restart.Inc()
}
