package suite

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
)

// GossipSubInspectorSuite encapsulates what is exposed to the libp2p node regarding the gossipsub RPC inspectors as
// well as their notification distributors.
type GossipSubInspectorSuite struct {
	component.Component
	aggregatedInspector       *AggregateRPCInspector
	ctrlMsgInspectDistributor p2p.GossipSubInspectorNotifDistributor
}

// NewGossipSubInspectorSuite creates a new GossipSubInspectorSuite.
// The suite is composed of the aggregated inspector, which is used to inspect the gossipsub rpc messages, and the
// control message notification distributor, which is used to notify consumers when a misbehaving peer regarding gossipsub
// control messages is detected.
// The suite is also a component, which is used to start and stop the rpc inspectors.
// Args:
//   - inspectors: the rpc inspectors that are used to inspect the gossipsub rpc messages.
//   - ctrlMsgInspectDistributor: the notification distributor that is used to notify consumers when a misbehaving peer
//
// regarding gossipsub control messages is detected.
// Returns:
//   - the new GossipSubInspectorSuite.
func NewGossipSubInspectorSuite(inspectors []p2p.GossipSubRPCInspector, ctrlMsgInspectDistributor p2p.GossipSubInspectorNotifDistributor) *GossipSubInspectorSuite {
	s := &GossipSubInspectorSuite{
		ctrlMsgInspectDistributor: ctrlMsgInspectDistributor,
		aggregatedInspector:       NewAggregateRPCInspector(inspectors...),
	}

	builder := component.NewComponentManagerBuilder()
	for _, inspector := range inspectors {
		inspector := inspector // capture loop variable
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			inspector.Start(ctx)

			select {
			case <-ctx.Done():
			case <-inspector.Ready():
				ready()
			}

			<-inspector.Done()
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

// AddInvalidCtrlMsgNotificationConsumer adds a consumer to the invalid control message notification distributor.
// This consumer is notified when a misbehaving peer regarding gossipsub control messages is detected. This follows a pub/sub
// pattern where the consumer is notified when a new notification is published.
// A consumer is only notified once for each notification, and only receives notifications that were published after it was added.
func (s *GossipSubInspectorSuite) AddInvCtrlMsgNotifConsumer(c p2p.GossipSubInvCtrlMsgNotifConsumer) {
	s.ctrlMsgInspectDistributor.AddConsumer(c)
}

// Inspectors returns all inspectors in the inspector suite.
func (s *GossipSubInspectorSuite) Inspectors() []p2p.GossipSubRPCInspector {
	return s.aggregatedInspector.Inspectors()
}
