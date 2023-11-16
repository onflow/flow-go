package inspector

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
)

// GossipSubInspectorSuite encapsulates what is exposed to the libp2p node regarding the gossipsub RPC inspectors as
// well as their notification distributors.
type GossipSubInspectorSuite struct {
	component.Component
	aggregatedInspector       *AggregateRPCInspector
	validationInspector       *validation.ControlMsgValidationInspector
	ctrlMsgInspectDistributor p2p.GossipSubInspectorNotifDistributor
}

var _ p2p.GossipSubInspectorSuite = (*GossipSubInspectorSuite)(nil)

// NewGossipSubInspectorSuite creates a new GossipSubInspectorSuite.
// The suite is composed of the aggregated inspector, which is used to inspect the gossipsub rpc messages, and the
// control message notification distributor, which is used to notify consumers when a misbehaving peer regarding gossipsub
// control messages is detected.
// The suite is also a component, which is used to start and stop the rpc inspectors.
// Args:
//   - metricsInspector: the control message metrics inspector.
//   - validationInspector: the gossipsub validation control message validation inspector.
//   - ctrlMsgInspectDistributor: the notification distributor that is used to notify consumers when a misbehaving peer
//
// regarding gossipsub control messages is detected.
// Returns:
//   - the new GossipSubInspectorSuite.
func NewGossipSubInspectorSuite(metricsInspector *inspector.ControlMsgMetricsInspector,
	validationInspector *validation.ControlMsgValidationInspector,
	ctrlMsgInspectDistributor p2p.GossipSubInspectorNotifDistributor) *GossipSubInspectorSuite {
	inspectors := []p2p.GossipSubRPCInspector{metricsInspector, validationInspector}
	s := &GossipSubInspectorSuite{
		ctrlMsgInspectDistributor: ctrlMsgInspectDistributor,
		validationInspector:       validationInspector,
		aggregatedInspector:       NewAggregateRPCInspector(inspectors...),
	}

	builder := component.NewComponentManagerBuilder()
	for _, rpcInspector := range inspectors {
		rpcInspector := rpcInspector // capture loop variable
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			rpcInspector.Start(ctx)

			select {
			case <-ctx.Done():
			case <-rpcInspector.Ready():
				ready()
			}

			<-rpcInspector.Done()
		})
	}

	s.Component = builder.Build()
	return s
}

// InspectFunc returns the inspect function that is used to inspect the gossipsub rpc messages.
// This function follows a dependency injection pattern, where the inspect function is injected into the gossipsu, and
// is called whenever a gossipsub rpc message is received.
func (s *GossipSubInspectorSuite) InspectFunc() func(peer.ID, *pubsub.RPC) error {
	return s.aggregatedInspector.Inspect
}

// AddInvalidControlMessageConsumer adds a consumer to the invalid control message notification distributor.
// This consumer is notified when a misbehaving peer regarding gossipsub control messages is detected. This follows a pub/sub
// pattern where the consumer is notified when a new notification is published.
// A consumer is only notified once for each notification, and only receives notifications that were published after it was added.
func (s *GossipSubInspectorSuite) AddInvalidControlMessageConsumer(c p2p.GossipSubInvCtrlMsgNotifConsumer) {
	s.ctrlMsgInspectDistributor.AddConsumer(c)
}

// ActiveClustersChanged is called when the list of active collection nodes cluster is changed.
// GossipSubInspectorSuite consumes this event and forwards it to all the respective rpc inspectors, that are
// concerned with this cluster-based topics (i.e., channels), so that they can update their internal state.
func (s *GossipSubInspectorSuite) ActiveClustersChanged(list flow.ChainIDList) {
	for _, rpcInspector := range s.aggregatedInspector.Inspectors() {
		if r, ok := rpcInspector.(p2p.GossipSubMsgValidationRpcInspector); ok {
			r.ActiveClustersChanged(list)
		}
	}
}
