package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

type GossipSubMetrics struct {
	receivedIHaveCount                  prometheus.Counter
	receivedIWantCount                  prometheus.Counter
	receivedGraftCount                  prometheus.Counter
	receivedPruneCount                  prometheus.Counter
	incomingRpcAcceptedFullyCount       prometheus.Counter
	incomingRpcAcceptedOnlyControlCount prometheus.Counter
	incomingRpcRejectedCount            prometheus.Counter
	receivedPublishMessageCount         prometheus.Counter

	prefix string
}

var _ module.GossipSubRpcInspectorMetrics = (*GossipSubMetrics)(nil)

func NewGossipSubMetrics(prefix string) *GossipSubMetrics {
	gs := &GossipSubMetrics{prefix: prefix}

	gs.receivedIHaveCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gs.prefix + "gossipsub_received_ihave_total",
			Help:      "number of received ihave messages from gossipsub protocol",
		},
	)

	gs.receivedIWantCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gs.prefix + "gossipsub_received_iwant_total",
			Help:      "number of received iwant messages from gossipsub protocol",
		},
	)

	gs.receivedGraftCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gs.prefix + "gossipsub_received_graft_total",
			Help:      "number of received graft messages from gossipsub protocol",
		},
	)

	gs.receivedPruneCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gs.prefix + "gossipsub_received_prune_total",
			Help:      "number of received prune messages from gossipsub protocol",
		},
	)

	gs.incomingRpcAcceptedFullyCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gs.prefix + "gossipsub_incoming_rpc_accepted_fully_total",
			Help:      "number of incoming rpc messages accepted fully by gossipsub protocol",
		},
	)

	gs.incomingRpcAcceptedOnlyControlCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gs.prefix + "gossipsub_incoming_rpc_accepted_only_control_total",
			Help:      "number of incoming rpc messages accepted only control messages by gossipsub protocol",
		},
	)

	gs.incomingRpcRejectedCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gs.prefix + "gossipsub_incoming_rpc_rejected_total",
			Help:      "number of incoming rpc messages rejected by gossipsub protocol",
		},
	)

	gs.receivedPublishMessageCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gs.prefix + "gossipsub_received_publish_message_total",
			Help:      "number of received publish messages from gossipsub protocol",
		},
	)

	return gs
}

// OnIWantReceived tracks the number of IWANT messages received by the node from other nodes.
// iWant is a control message that is sent by a node to request a message that it has seen advertised in an iHAVE message.
func (nc *GossipSubMetrics) OnIWantReceived(count int) {
	nc.receivedIWantCount.Add(float64(count))
}

// OnIHaveReceived tracks the number of IHAVE messages received by the node from other nodes.
// iHave is a control message that is sent by a node to another node to indicate that it has a new gossiped message.
func (nc *GossipSubMetrics) OnIHaveReceived(count int) {
	nc.receivedIHaveCount.Add(float64(count))
}

// OnGraftReceived tracks the number of GRAFT messages received by the node from other nodes.
// GRAFT is a control message of GossipSub protocol that connects two nodes over a topic directly as gossip partners.
func (nc *GossipSubMetrics) OnGraftReceived(count int) {
	nc.receivedGraftCount.Add(float64(count))
}

// OnPruneReceived tracks the number of PRUNE messages received by the node from other nodes.
// PRUNE is a control message of GossipSub protocol that disconnects two nodes over a topic.
func (nc *GossipSubMetrics) OnPruneReceived(count int) {
	nc.receivedPruneCount.Add(float64(count))
}

// OnIncomingRpcAcceptedFully tracks the number of RPC messages received by the node that are fully accepted.
// An RPC may contain any number of control messages, i.e., IHAVE, IWANT, GRAFT, PRUNE, as well as the actual messages.
// A fully accepted RPC means that all the control messages are accepted and all the messages are accepted.
func (nc *GossipSubMetrics) OnIncomingRpcAcceptedFully() {
	nc.incomingRpcAcceptedFullyCount.Inc()
}

// OnIncomingRpcAcceptedOnlyForControlMessages tracks the number of RPC messages received by the node that are accepted
// only for the control messages, i.e., only for the included IHAVE, IWANT, GRAFT, PRUNE. However, the actual messages
// included in the RPC are not accepted.
// This happens mostly when the validation pipeline of GossipSub is throttled, and cannot accept more actual messages for
// validation.
func (nc *GossipSubMetrics) OnIncomingRpcAcceptedOnlyForControlMessages() {
	nc.incomingRpcAcceptedOnlyControlCount.Inc()
}

// OnIncomingRpcRejected tracks the number of RPC messages received by the node that are rejected.
// This happens mostly when the RPC is coming from a low-scored peer based on the peer scoring module of GossipSub.
func (nc *GossipSubMetrics) OnIncomingRpcRejected() {
	nc.incomingRpcRejectedCount.Inc()
}

// OnPublishedGossipMessagesReceived tracks the number of gossip messages received by the node from other nodes over an
// RPC message.
func (nc *GossipSubMetrics) OnPublishedGossipMessagesReceived(count int) {
	nc.receivedPublishMessageCount.Add(float64(count))
}
