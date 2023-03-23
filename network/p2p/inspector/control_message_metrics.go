package inspector

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/network/p2p"
)

// ControlMsgMetricsInspector a  GossipSub RPC inspector that will observe incoming RPC's and collect metrics related to control messages.
type ControlMsgMetricsInspector struct {
	component.Component
	metrics p2p.GossipSubControlMetricsObserver
}

var _ p2p.GossipSubRPCInspector = (*ControlMsgMetricsInspector)(nil)

func (c *ControlMsgMetricsInspector) Inspect(from peer.ID, rpc *pubsub.RPC) error {
	c.metrics.ObserveRPC(from, rpc)
	return nil
}

// NewControlMsgMetricsInspector returns a new *ControlMsgMetricsInspector
func NewControlMsgMetricsInspector(metrics p2p.GossipSubControlMetricsObserver) *ControlMsgMetricsInspector {
	return &ControlMsgMetricsInspector{
		Component: component.NewComponentManagerBuilder().Build(),
		metrics:   metrics,
	}
}
