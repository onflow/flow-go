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

var _ module.GossipSubRouterMetrics = (*GossipSubMetrics)(nil)

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

// LocalGossipSubRouterMetrics encapsulates the metrics collectors for GossipSub router of the local node.
// It gives a lens into the local node's view of the GossipSub protocol.
type LocalGossipSubRouterMetrics struct {
	localMeshSize                 prometheus.GaugeVec
	peerAddedOnProtocolCount      prometheus.CounterVec
	peerRemovedFromProtocolCount  prometheus.Counter
	localPeerJoinedTopicCount     prometheus.CounterVec
	localPeerLeftTopicCount       prometheus.CounterVec
	peerGraftTopicCount           prometheus.CounterVec
	peerPruneTopicCount           prometheus.CounterVec
	messageEnteredValidationCount prometheus.Counter
	messageDeliveredCount         prometheus.Counter
	messageRejectedCount          prometheus.CounterVec
	messageDuplicateSize          prometheus.Histogram
	peerThrottledCount            prometheus.Counter
	rpcRecCount                   prometheus.Counter
	rpcSentCount                  prometheus.Counter
	rpcDroppedCount               prometheus.Counter
	undeliveredMessageCount       prometheus.Counter
}

func NewGossipSubLocalMeshMetrics(prefix string) *LocalGossipSubRouterMetrics {
	return &LocalGossipSubRouterMetrics{
		localMeshSize: *promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespaceNetwork,
				Subsystem: subsystemGossip,
				Name:      prefix + "gossipsub_local_mesh_size",
				Help:      "number of peers in the local mesh of the node",
			},
			[]string{LabelChannel},
		),
		peerAddedOnProtocolCount: *promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_added_peer_on_protocol_total",
			Help:      "number of peers added to the local gossipsub router on a gossipsub protocol",
		}, []string{LabelProtocol}),
		peerRemovedFromProtocolCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_removed_peer_total",
			Help:      "number of peers removed from the local gossipsub router on a gossipsub protocol due to unavailability or blacklisting",
		}),
		localPeerJoinedTopicCount: *promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_joined_topic_total",
			Help:      "number of times the local node joined (i.e., subscribed) to a topic",
		}, []string{LabelChannel}),
		localPeerLeftTopicCount: *promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_left_topic_total",
			Help:      "number of times the local node left (i.e., unsubscribed) from a topic",
		}, []string{LabelChannel}),
		peerGraftTopicCount: *promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_graft_topic_total",
			Help:      "number of peers grafted to a topic on the local mesh of the node",
		}, []string{LabelChannel}),
		peerPruneTopicCount: *promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_prune_topic_total",
			Help:      "number of peers pruned from a topic on the local mesh of the node",
		}, []string{LabelChannel}),
		messageEnteredValidationCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_message_entered_validation_total",
			Help:      "number of messages entered internal validation pipeline of gossipsub",
		}),
		messageDeliveredCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_message_delivered_total",
			Help:      "number of messages delivered to all subscribers of the topic",
		}),
		messageRejectedCount: *promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_message_rejected_total",
			Help:      "number of messages rejected by the validation pipeline",
		}, []string{LabelRejectionReason}),
		messageDuplicateSize: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Buckets:   []float64{KiB, 100 * KiB, 1 * MiB},
			Name:      prefix + "gossipsub_duplicate_message_size_bytes",
			Help:      "size of messages that are duplicates of already received messages",
		}),
		peerThrottledCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_peer_throttled_total",
			Help:      "number of peers that are throttled by the local node, i.e., the local node is not accepting any pubsub message from the peer but may still accept control messages",
		}),
		rpcRecCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_rpc_received_total",
			Help:      "number of rpc messages received and processed by the router (i.e., passed rpc inspection)",
		}),
		rpcSentCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_rpc_sent_total",
			Help:      "number of rpc messages sent by the router",
		}),
		rpcDroppedCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_rpc_dropped_total",
			Help:      "number of outbound rpc messages dropped, typically because the outbound message queue is full",
		}),
		undeliveredMessageCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_undelivered_message_total",
			Help:      "number of undelivered messages, i.e., messages that are not delivered to at least one subscriber",
		}),
	}
}

var _ module.LocalGossipSubRouterMetrics = (*LocalGossipSubRouterMetrics)(nil)

// OnLocalMeshSizeUpdated updates the local mesh size metric.
func (g *LocalGossipSubRouterMetrics) OnLocalMeshSizeUpdated(topic string, size int) {
	g.localMeshSize.WithLabelValues(topic).Set(float64(size))
}

// OnPeerAddedToProtocol is called when the local node receives a stream from a peer on a gossipsub-related protocol.
// Args:
//
//	protocol: the protocol name that the peer is connected to.
func (g *LocalGossipSubRouterMetrics) OnPeerAddedToProtocol(protocol string) {
	g.peerAddedOnProtocolCount.WithLabelValues(protocol).Inc()
}

// OnPeerRemovedFromProtocol is called when the local considers a remote peer blacklisted or unavailable.
func (g *LocalGossipSubRouterMetrics) OnPeerRemovedFromProtocol() {
	g.peerRemovedFromProtocolCount.Inc()
}

// OnLocalPeerJoinedTopic is called when the local node subscribes to a gossipsub topic.
// Args:
//
//	topic: the topic that the local peer subscribed to.
func (g *LocalGossipSubRouterMetrics) OnLocalPeerJoinedTopic(topic string) {
	g.localPeerJoinedTopicCount.WithLabelValues(topic).Inc()
}

// OnLocalPeerLeftTopic is called when the local node unsubscribes from a gossipsub topic.
// Args:
//
//	topic: the topic that the local peer has unsubscribed from.
func (g *LocalGossipSubRouterMetrics) OnLocalPeerLeftTopic(topic string) {
	g.localPeerLeftTopicCount.WithLabelValues(topic).Inc()
}

// OnPeerGraftTopic is called when the local node receives a GRAFT message from a remote peer on a topic.
// Note: the received GRAFT at this point is considered passed the RPC inspection, and is accepted by the local node.
func (g *LocalGossipSubRouterMetrics) OnPeerGraftTopic(topic string) {
	g.peerGraftTopicCount.WithLabelValues(topic).Inc()
}

// OnPeerPruneTopic is called when the local node receives a PRUNE message from a remote peer on a topic.
// Note: the received PRUNE at this point is considered passed the RPC inspection, and is accepted by the local node.
func (g *LocalGossipSubRouterMetrics) OnPeerPruneTopic(topic string) {
	g.peerPruneTopicCount.WithLabelValues(topic).Inc()
}

// OnMessageEnteredValidation is called when a received pubsub message enters the validation pipeline. It is the
// internal validation pipeline of GossipSub protocol. The message may be rejected or accepted by the validation
// pipeline.
func (g *LocalGossipSubRouterMetrics) OnMessageEnteredValidation() {
	g.messageEnteredValidationCount.Inc()
}

// OnMessageRejected is called when a received pubsub message is rejected by the validation pipeline.
// Args:
//
//	reason: the reason for rejection.
func (g *LocalGossipSubRouterMetrics) OnMessageRejected(int size, reason string) {
	g.messageRejectedCount.WithLabelValues(reason).Inc()
}

// OnMessageDuplicate is called when a received pubsub message is a duplicate of a previously received message, and
// is dropped.
// Args:
//
//	size: the size of the duplicate message.
func (g *LocalGossipSubRouterMetrics) OnMessageDuplicate(size int) {
	g.messageDuplicateSize.Observe(float64(size))
}

// OnPeerThrottled is called when a peer is throttled by the local node, i.e., the local node is not accepting any
// pubsub message from the peer but may still accept control messages.
func (g *LocalGossipSubRouterMetrics) OnPeerThrottled() {
	g.peerThrottledCount.Inc()
}

// OnRpcReceived is called when an RPC message is received by the local node. The received RPC is considered
// passed the RPC inspection, and is accepted by the local node.
func (g *LocalGossipSubRouterMetrics) OnRpcReceived(int, int, int, int, int) {
	g.rpcRecCount.Inc()
}

// OnRpcSent is called when an RPC message is sent by the local node.
// Note: the sent RPC is considered passed the RPC inspection, and is accepted by the local node.
func (g *LocalGossipSubRouterMetrics) OnRpcSent(msgCount int, iHaveCount int, iWantCount int, graftCount int, pruneCount int) {
	g.rpcSentCount.Inc()
}

// OnRpcDropped is called when an outbound RPC message is dropped by the local node, typically because the local node
// outbound message queue is full; or the RPC is big and the local node cannot fragment it.
func (g *LocalGossipSubRouterMetrics) OnRpcDropped(msgCount int, iHaveCount int, iWantCount int, graftCount int, pruneCount int) {
	g.rpcDroppedCount.Inc()
}

// OnUndeliveredMessage is called when a message is not delivered at least one subscriber of the topic, for example when
// the subscriber is too slow to process the message.
func (g *LocalGossipSubRouterMetrics) OnUndeliveredMessage() {
	g.undeliveredMessageCount.Inc()
}

// OnMessageDelivered is called when a message is delivered to all subscribers of the topic.
func (g *LocalGossipSubRouterMetrics) OnMessageDelivered() {
	g.messageDeliveredCount.Inc()
}
