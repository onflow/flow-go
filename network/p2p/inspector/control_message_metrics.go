package inspector

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p/p2pnode"
)

// ControlMsgMetricsInspector a  GossipSub RPC inspector that will observe incoming RPC's and collect metrics related to control messages.
type ControlMsgMetricsInspector struct {
	metrics *p2pnode.GossipSubControlMessageMetrics
}

func (c *ControlMsgMetricsInspector) Inspect(from peer.ID, rpc *pubsub.RPC) error {
	c.metrics.ObserveRPC(from, rpc)
	return nil
}

// NewControlMsgMetricsInspector returns a new *ControlMsgMetricsInspector
func NewControlMsgMetricsInspector(metrics *p2pnode.GossipSubControlMessageMetrics) *ControlMsgMetricsInspector {
	return &ControlMsgMetricsInspector{
		metrics: metrics,
	}
}
