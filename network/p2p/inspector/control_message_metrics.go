package inspector

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/network/p2p"
)

const (
	// rpcInspectorComponentName the rpc inspector component name.
	rpcInspectorComponentName = "gossipsub_rpc_metrics_observer_inspector"
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

// Name returns the name of the rpc inspector.
func (c *ControlMsgMetricsInspector) Name() string {
	return rpcInspectorComponentName
}

// NewControlMsgMetricsInspector returns a new *ControlMsgMetricsInspector
func NewControlMsgMetricsInspector(metrics p2p.GossipSubControlMetricsObserver) *ControlMsgMetricsInspector {
	return &ControlMsgMetricsInspector{
		Component: &module.NoopReadyDoneAware{},
		metrics:   metrics,
	}
}
