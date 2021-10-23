package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/model/flow"
)

type PingCollector struct {
	reachable       *prometheus.GaugeVec
	sealedHeight    *prometheus.GaugeVec
	hotstuffCurView *prometheus.GaugeVec
}

func NewPingCollector() *PingCollector {
	pc := &PingCollector{
		reachable: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "node_reachable",
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Help:      "report whether a node is reachable",
		}, []string{LabelNodeID, LabelNodeAddress, LabelNodeRole, LabelNodeInfo}),
		sealedHeight: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "sealed_height",
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Help:      "the last sealed height of a node",
		}, []string{LabelNodeID, LabelNodeAddress, LabelNodeRole, LabelNodeInfo, LabelNodeVersion}),
		hotstuffCurView: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "hotstuff_curview",
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Help:      "the hotstuff current view",
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

func (pc *PingCollector) NodeInfo(node *flow.Identity, nodeInfo string, version string, sealedHeight uint64, hotstuffCurView uint64) {
	pc.sealedHeight.With(prometheus.Labels{
		LabelNodeID:      node.NodeID.String(),
		LabelNodeAddress: node.Address,
		LabelNodeRole:    node.Role.String(),
		LabelNodeInfo:    nodeInfo,
		LabelNodeVersion: version}).
		Set(float64(sealedHeight))

	pc.hotstuffCurView.With(prometheus.Labels{
		LabelNodeID:      node.NodeID.String(),
		LabelNodeAddress: node.Address,
		LabelNodeInfo:    nodeInfo,
	}).
		Set(float64(hotstuffCurView))
}
