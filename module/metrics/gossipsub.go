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
	// localMeshSize is the number of peers in the local mesh of the node on each topic.
	localMeshSize prometheus.GaugeVec

	// peerAddedOnProtocolCount is the number of peers added to the local gossipsub router on a gossipsub protocol.
	peerAddedOnProtocolCount prometheus.CounterVec

	// peerRemovedFromProtocolCount is the number of peers removed from the local gossipsub router (i.e., blacklisted or unavailable).
	peerRemovedFromProtocolCount prometheus.Counter

	// localPeerJoinedTopicCount is the number of times the local node joined (i.e., subscribed) to a topic.
	localPeerJoinedTopicCount prometheus.Counter

	// localPeerLeftTopicCount is the number of times the local node left (i.e., unsubscribed) from a topic.
	localPeerLeftTopicCount prometheus.Counter

	// peerGraftTopicCount is the number of peers grafted to a topic on the local mesh of the node, i.e., the local node
	// is directly connected to the peer on the topic, and exchange messages directly.
	peerGraftTopicCount prometheus.CounterVec

	// peerPruneTopicCount is the number of peers pruned from a topic on the local mesh of the node, i.e., the local node
	// is no longer directly connected to the peer on the topic, and exchange messages indirectly.
	peerPruneTopicCount prometheus.CounterVec

	// messageEnteredValidationCount is the number of incoming pubsub messages entered internal validation pipeline of gossipsub.
	messageEnteredValidationCount prometheus.Counter

	// messageDeliveredSize is the size of messages delivered to all subscribers of the topic.
	messageDeliveredSize prometheus.Histogram

	// messageRejectedSize is the size of inbound messages rejected by the validation pipeline; the rejection reason is also included.
	messageRejectedSize prometheus.HistogramVec

	// messageDuplicateSize is the size of messages that are duplicates of already received messages.
	messageDuplicateSize prometheus.Histogram

	// peerThrottledCount is the number of peers that are throttled by the local node, i.e., the local node is not accepting
	// any pubsub message from the peer but may still accept control messages.
	peerThrottledCount prometheus.Counter

	// rpcRcvCount is the number of rpc messages received and processed by the router (i.e., passed rpc inspection).
	rpcRcvCount prometheus.Counter

	// iWantRcvCount is the number of iwant messages received by the router on rpcs.
	iWantRcvCount prometheus.Counter

	// iHaveRcvCount is the number of ihave messages received by the router on rpcs.
	iHaveRcvCount prometheus.Counter

	// graftRcvCount is the number of graft messages received by the router on rpcs.
	graftRcvCount prometheus.Counter

	// pruneRcvCount is the number of prune messages received by the router on rpcs.
	pruneRcvCount prometheus.Counter

	// pubsubMsgRcvCount is the number of pubsub messages received by the router.
	pubsubMsgRcvCount prometheus.Counter

	// rpcSentCount is the number of rpc messages sent by the router.
	rpcSentCount prometheus.Counter

	// iWantSentCount is the number of iwant messages sent by the router on rpcs.
	iWantSentCount prometheus.Counter

	// iHaveSentCount is the number of ihave messages sent by the router on rpcs.
	iHaveSentCount prometheus.Counter

	// graftSentCount is the number of graft messages sent by the router on rpcs.
	graftSentCount prometheus.Counter

	// pruneSentCount is the number of prune messages sent by the router on rpcs.
	pruneSentCount prometheus.Counter

	// pubsubMsgSentCount is the number of pubsub messages sent by the router.
	pubsubMsgSentCount prometheus.Counter

	// outboundRpcDroppedCount is the number of outbound rpc messages dropped, typically because the outbound message queue is full.
	outboundRpcDroppedCount prometheus.Counter

	// undeliveredOutboundMessageCount is the number of undelivered messages, i.e., messages that are not delivered to at least one subscriber.
	undeliveredOutboundMessageCount prometheus.Counter
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
		localPeerJoinedTopicCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_joined_topic_total",
			Help:      "number of times the local node joined (i.e., subscribed) to a topic",
		}),
		localPeerLeftTopicCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_left_topic_total",
			Help:      "number of times the local node left (i.e., unsubscribed) from a topic",
		}),
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
		messageDeliveredSize: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Buckets:   []float64{KiB, 100 * KiB, 1 * MiB},
			Name:      prefix + "gossipsub_message_delivered_size",
			Help:      "size of messages delivered to all subscribers of the topic",
		}),
		messageRejectedSize: *promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_message_rejected_size_bytes",
			Help:      "size of messages rejected by the validation pipeline",
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
		rpcRcvCount: prometheus.NewCounter(prometheus.CounterOpts{
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
		outboundRpcDroppedCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_rpc_dropped_total",
			Help:      "number of outbound rpc messages dropped, typically because the outbound message queue is full",
		}),
		undeliveredOutboundMessageCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_undelivered_message_total",
			Help:      "number of undelivered messages, i.e., messages that are not delivered to at least one subscriber",
		}),
		iHaveRcvCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_ihave_received_total",
			Help:      "number of ihave messages received by the router on rpcs",
		}),
		iWantRcvCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_iwant_received_total",
			Help:      "number of iwant messages received by the router on rpcs",
		}),
		graftRcvCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_graft_received_total",
			Help:      "number of graft messages received by the router on rpcs",
		}),
		pruneRcvCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_prune_received_total",
			Help:      "number of prune messages received by the router on rpcs",
		}),
		pubsubMsgRcvCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_pubsub_message_received_total",
			Help:      "number of pubsub messages received by the router",
		}),
		iHaveSentCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_ihave_sent_total",
			Help:      "number of ihave messages sent by the router on rpcs",
		}),
		iWantSentCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_iwant_sent_total",
			Help:      "number of iwant messages sent by the router on rpcs",
		}),
		graftSentCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_graft_sent_total",
			Help:      "number of graft messages sent by the router on rpcs",
		}),
		pruneSentCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_prune_sent_total",
			Help:      "number of prune messages sent by the router on rpcs",
		}),
		pubsubMsgSentCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_pubsub_message_sent_total",
			Help:      "number of pubsub messages sent by the router",
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
func (g *LocalGossipSubRouterMetrics) OnLocalPeerJoinedTopic() {
	g.localPeerJoinedTopicCount.Inc()
}

// OnLocalPeerLeftTopic is called when the local node unsubscribes from a gossipsub topic.
// Args:
//
//	topic: the topic that the local peer has unsubscribed from.
func (g *LocalGossipSubRouterMetrics) OnLocalPeerLeftTopic() {
	g.localPeerLeftTopicCount.Inc()
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
func (g *LocalGossipSubRouterMetrics) OnMessageEnteredValidation(int) {
	g.messageEnteredValidationCount.Inc()
}

// OnMessageRejected is called when a received pubsub message is rejected by the validation pipeline.
// Args:
//
//	reason: the reason for rejection.
//	size: the size of the rejected message.
func (g *LocalGossipSubRouterMetrics) OnMessageRejected(size int, reason string) {
	g.messageRejectedSize.WithLabelValues(reason).Observe(float64(size))
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
func (g *LocalGossipSubRouterMetrics) OnRpcReceived(msgCount int, iHaveCount int, iWantCount int, graftCount int, pruneCount int) {
	g.rpcRcvCount.Inc()
	g.pubsubMsgRcvCount.Add(float64(msgCount))
	g.iHaveRcvCount.Add(float64(iHaveCount))
	g.iWantRcvCount.Add(float64(iWantCount))
	g.graftRcvCount.Add(float64(graftCount))
	g.pruneRcvCount.Add(float64(pruneCount))
}

// OnRpcSent is called when an RPC message is sent by the local node.
// Note: the sent RPC is considered passed the RPC inspection, and is accepted by the local node.
func (g *LocalGossipSubRouterMetrics) OnRpcSent(msgCount int, iHaveCount int, iWantCount int, graftCount int, pruneCount int) {
	g.rpcSentCount.Inc()
	g.pubsubMsgSentCount.Add(float64(msgCount))
	g.iHaveSentCount.Add(float64(iHaveCount))
	g.iWantSentCount.Add(float64(iWantCount))
	g.graftSentCount.Add(float64(graftCount))
	g.pruneSentCount.Add(float64(pruneCount))
}

// OnOutboundRpcDropped is called when an outbound RPC message is dropped by the local node, typically because the local node
// outbound message queue is full; or the RPC is big and the local node cannot fragment it.
func (g *LocalGossipSubRouterMetrics) OnOutboundRpcDropped() {
	g.outboundRpcDroppedCount.Inc()
}

// OnUndeliveredMessage is called when a message is not delivered at least one subscriber of the topic, for example when
// the subscriber is too slow to process the message.
func (g *LocalGossipSubRouterMetrics) OnUndeliveredMessage() {
	g.undeliveredOutboundMessageCount.Inc()
}

// OnMessageDeliveredToAllSubscribers is called when a message is delivered to all subscribers of the topic.
// Args:
//
//	size: the size of the delivered message.
func (g *LocalGossipSubRouterMetrics) OnMessageDeliveredToAllSubscribers(size int) {
	g.messageDeliveredSize.Observe(float64(size))
}
