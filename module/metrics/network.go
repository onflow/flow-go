package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

const (
	_   = iota
	KiB = 1 << (10 * iota)
	MiB
	GiB
)

type NetworkCollector struct {
	outboundMessageSize          *prometheus.HistogramVec
	inboundMessageSize           *prometheus.HistogramVec
	duplicateMessagesDropped     *prometheus.CounterVec
	queueSize                    *prometheus.GaugeVec
	queueDuration                *prometheus.HistogramVec
	numMessagesProcessing        *prometheus.GaugeVec
	numDirectMessagesSending     *prometheus.GaugeVec
	inboundProcessTime           *prometheus.CounterVec
	outboundConnectionCount      prometheus.Gauge
	inboundConnectionCount       prometheus.Gauge
	dnsLookupDuration            prometheus.Histogram
	dnsCacheMissCount            prometheus.Counter
	dnsCacheHitCount             prometheus.Counter
	dnsCacheInvalidationCount    prometheus.Counter
	dnsLookupRequestDroppedCount prometheus.Counter
	routingTableSize             prometheus.Gauge

	// TODO: encapsulate these in a separate GossipSub collector.
	gossipSubReceivedIHaveCount                  prometheus.Counter
	gossipSubReceivedIWantCount                  prometheus.Counter
	gossipSubReceivedGraftCount                  prometheus.Counter
	gossipSubReceivedPruneCount                  prometheus.Counter
	gossipSubIncomingRpcAcceptedFullyCount       prometheus.Counter
	gossipSubIncomingRpcAcceptedOnlyControlCount prometheus.Counter
	gossipSubIncomingRpcRejectedCount            prometheus.Counter

	// authorization, rate limiting metrics
	unAuthorizedMessagesCount       *prometheus.CounterVec
	rateLimitedUnicastMessagesCount *prometheus.CounterVec

	prefix string
}

var _ module.NetworkMetrics = (*NetworkCollector)(nil)

type NetworkCollectorOpt func(*NetworkCollector)

func WithNetworkPrefix(prefix string) NetworkCollectorOpt {
	return func(nc *NetworkCollector) {
		if prefix != "" {
			nc.prefix = prefix + "_"
		}
	}
}

func NewNetworkCollector(opts ...NetworkCollectorOpt) *NetworkCollector {
	nc := &NetworkCollector{}

	for _, opt := range opts {
		opt(nc)
	}

	nc.outboundMessageSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "outbound_message_size_bytes",
			Help:      "size of the outbound network message",
			Buckets:   []float64{KiB, 100 * KiB, 500 * KiB, 1 * MiB, 2 * MiB, 4 * MiB},
		}, []string{LabelChannel, LabelMessage},
	)

	nc.inboundMessageSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "inbound_message_size_bytes",
			Help:      "size of the inbound network message",
			Buckets:   []float64{KiB, 100 * KiB, 500 * KiB, 1 * MiB, 2 * MiB, 4 * MiB},
		}, []string{LabelChannel, LabelMessage},
	)

	nc.duplicateMessagesDropped = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "duplicate_messages_dropped",
			Help:      "number of duplicate messages dropped",
		}, []string{LabelChannel, LabelMessage},
	)

	nc.dnsLookupDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "dns_lookup_duration_ms",
			Buckets:   []float64{1, 10, 100, 500, 1000, 2000},
			Help:      "the time spent on resolving a dns lookup (including cache hits)",
		},
	)

	nc.dnsCacheMissCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "dns_cache_miss_total",
			Help:      "the number of dns lookups that miss the cache and made through network",
		},
	)

	nc.dnsCacheInvalidationCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "dns_cache_invalidation_total",
			Help:      "the number of times dns cache is invalidated for an entry",
		},
	)

	nc.dnsCacheHitCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "dns_cache_hit_total",
			Help:      "the number of dns cache hits",
		},
	)

	nc.dnsLookupRequestDroppedCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "dns_lookup_requests_dropped_total",
			Help:      "the number of dns lookup requests dropped",
		},
	)

	nc.queueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemQueue,
			Name:      nc.prefix + "message_queue_size",
			Help:      "the number of elements in the message receive queue",
		}, []string{LabelPriority},
	)

	nc.queueDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemQueue,
			Name:      nc.prefix + "message_queue_duration_seconds",
			Help:      "duration [seconds; measured with float64 precision] of how long a message spent in the queue before delivered to an engine.",
			Buckets:   []float64{0.01, 0.1, 0.5, 1, 2, 5}, // 10ms, 100ms, 500ms, 1s, 2s, 5s
		}, []string{LabelPriority},
	)

	nc.numMessagesProcessing = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemQueue,
			Name:      nc.prefix + "current_messages_processing",
			Help:      "the number of messages currently being processed",
		}, []string{LabelChannel},
	)

	nc.numDirectMessagesSending = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "direct_messages_in_progress",
			Help:      "the number of direct messages currently in the process of sending",
		}, []string{LabelChannel},
	)

	nc.inboundProcessTime = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemQueue,
			Name:      nc.prefix + "engine_message_processing_time_seconds",
			Help:      "duration [seconds; measured with float64 precision] of how long a queue worker blocked for an engine processing message",
		}, []string{LabelChannel},
	)

	nc.outboundConnectionCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemQueue,
			Name:      nc.prefix + "outbound_connection_count",
			Help:      "the number of outbound connections of this node",
		},
	)

	nc.inboundConnectionCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemQueue,
			Name:      nc.prefix + "inbound_connection_count",
			Help:      "the number of inbound connections of this node",
		},
	)

	nc.routingTableSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name:      nc.prefix + "routing_table_size",
			Namespace: namespaceNetwork,
			Subsystem: subsystemDHT,
			Help:      "the size of the DHT routing table",
		},
	)

	nc.unAuthorizedMessagesCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemAuth,
			Name:      nc.prefix + "unauthorized_messages_count",
			Help:      "number of messages that failed authorization validation",
		}, []string{LabelNodeRole, LabelMessage, LabelChannel, LabelViolationReason},
	)

	nc.rateLimitedUnicastMessagesCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemRateLimiting,
			Name:      nc.prefix + "rate_limited_unicast_messages_count",
			Help:      "number of messages sent via unicast that have been rate limited",
		}, []string{LabelNodeRole, LabelMessage, LabelChannel, LabelRateLimitReason},
	)

	nc.gossipSubReceivedIHaveCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "gossipsub_received_ihave_total",
			Help:      "number of received ihave messages from gossipsub protocol",
		},
	)

	nc.gossipSubReceivedIWantCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "gossipsub_received_iwant_total",
			Help:      "number of received iwant messages from gossipsub protocol",
		},
	)

	nc.gossipSubReceivedGraftCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "gossipsub_received_graft_total",
			Help:      "number of received graft messages from gossipsub protocol",
		},
	)

	nc.gossipSubReceivedPruneCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "gossipsub_received_prune_total",
			Help:      "number of received prune messages from gossipsub protocol",
		},
	)

	nc.gossipSubIncomingRpcAcceptedFullyCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "gossipsub_incoming_rpc_accepted_fully_total",
			Help:      "number of incoming rpc messages accepted fully by gossipsub protocol",
		},
	)

	nc.gossipSubIncomingRpcAcceptedOnlyControlCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "gossipsub_incoming_rpc_accepted_only_control_total",
			Help:      "number of incoming rpc messages accepted only control messages by gossipsub protocol",
		},
	)

	nc.gossipSubIncomingRpcRejectedCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "gossipsub_incoming_rpc_rejected_total",
			Help:      "number of incoming rpc messages rejected by gossipsub protocol",
		},
	)

	return nc
}

// NetworkMessageSent tracks the message size of the last message sent out on the wire
// in bytes for the given topic
func (nc *NetworkCollector) NetworkMessageSent(sizeBytes int, topic string, messageType string) {
	nc.outboundMessageSize.WithLabelValues(topic, messageType).Observe(float64(sizeBytes))
}

// NetworkMessageReceived tracks the message size of the last message received on the wire
// in bytes for the given topic
func (nc *NetworkCollector) NetworkMessageReceived(sizeBytes int, topic string, messageType string) {
	nc.inboundMessageSize.WithLabelValues(topic, messageType).Observe(float64(sizeBytes))
}

// NetworkDuplicateMessagesDropped tracks the number of messages dropped by the network layer due to duplication
func (nc *NetworkCollector) NetworkDuplicateMessagesDropped(topic, messageType string) {
	nc.duplicateMessagesDropped.WithLabelValues(topic, messageType).Add(1)
}

func (nc *NetworkCollector) MessageAdded(priority int) {
	nc.queueSize.WithLabelValues(strconv.Itoa(priority)).Inc()
}

func (nc *NetworkCollector) MessageRemoved(priority int) {
	nc.queueSize.WithLabelValues(strconv.Itoa(priority)).Dec()
}

func (nc *NetworkCollector) QueueDuration(duration time.Duration, priority int) {
	nc.queueDuration.WithLabelValues(strconv.Itoa(priority)).Observe(duration.Seconds())
}

func (nc *NetworkCollector) MessageProcessingStarted(topic string) {
	nc.numMessagesProcessing.WithLabelValues(topic).Inc()
}

func (nc *NetworkCollector) DirectMessageStarted(topic string) {
	nc.numDirectMessagesSending.WithLabelValues(topic).Inc()
}

func (nc *NetworkCollector) DirectMessageFinished(topic string) {
	nc.numDirectMessagesSending.WithLabelValues(topic).Dec()
}

func (nc *NetworkCollector) RoutingTablePeerAdded() {
	nc.routingTableSize.Inc()
}

func (nc *NetworkCollector) RoutingTablePeerRemoved() {
	nc.routingTableSize.Dec()
}

// MessageProcessingFinished tracks the time a queue worker blocked by an engine for processing an incoming message on specified topic (i.e., channel).
func (nc *NetworkCollector) MessageProcessingFinished(topic string, duration time.Duration) {
	nc.numMessagesProcessing.WithLabelValues(topic).Dec()
	nc.inboundProcessTime.WithLabelValues(topic).Add(duration.Seconds())
}

// OutboundConnections updates the metric tracking the number of outbound connections of this node
func (nc *NetworkCollector) OutboundConnections(connectionCount uint) {
	nc.outboundConnectionCount.Set(float64(connectionCount))
}

// InboundConnections updates the metric tracking the number of inbound connections of this node
func (nc *NetworkCollector) InboundConnections(connectionCount uint) {
	nc.inboundConnectionCount.Set(float64(connectionCount))
}

// DNSLookupDuration tracks the time spent to resolve a DNS address.
func (nc *NetworkCollector) DNSLookupDuration(duration time.Duration) {
	nc.dnsLookupDuration.Observe(float64(duration.Milliseconds()))
}

// OnDNSCacheMiss tracks the total number of dns requests resolved through looking up the network.
func (nc *NetworkCollector) OnDNSCacheMiss() {
	nc.dnsCacheMissCount.Inc()
}

// OnDNSCacheInvalidated is called whenever dns cache is invalidated for an entry
func (nc *NetworkCollector) OnDNSCacheInvalidated() {
	nc.dnsCacheInvalidationCount.Inc()
}

// OnDNSCacheHit tracks the total number of dns requests resolved through the cache without
// looking up the network.
func (nc *NetworkCollector) OnDNSCacheHit() {
	nc.dnsCacheHitCount.Inc()
}

// OnDNSLookupRequestDropped tracks the number of dns lookup requests that are dropped due to a full queue
func (nc *NetworkCollector) OnDNSLookupRequestDropped() {
	nc.dnsLookupRequestDroppedCount.Inc()
}

// OnUnauthorizedMessage tracks the number of unauthorized messages seen on the network.
func (nc *NetworkCollector) OnUnauthorizedMessage(role, msgType, topic, offense string) {
	nc.unAuthorizedMessagesCount.WithLabelValues(role, msgType, topic, offense).Inc()
}

// OnRateLimitedUnicastMessage tracks the number of rate limited messages seen on the network.
func (nc *NetworkCollector) OnRateLimitedUnicastMessage(role, msgType, topic, reason string) {
	nc.rateLimitedUnicastMessagesCount.WithLabelValues(role, msgType, topic, reason).Inc()
}

// OnIWantReceived tracks the number of IWANT messages received by the node from other nodes.
// iWant is a control message that is sent by a node to request a message that it has seen advertised in an iHAVE message.
func (nc *NetworkCollector) OnIWantReceived(count int) {
	nc.gossipSubReceivedIWantCount.Add(float64(count))
}

// OnIHaveReceived tracks the number of IHAVE messages received by the node from other nodes.
// iHave is a control message that is sent by a node to another node to indicate that it has a new gossiped message.
func (nc *NetworkCollector) OnIHaveReceived(count int) {
	nc.gossipSubReceivedIHaveCount.Add(float64(count))
}

// OnGraftReceived tracks the number of GRAFT messages received by the node from other nodes.
// GRAFT is a control message of GossipSub protocol that connects two nodes over a topic directly as gossip partners.
func (nc *NetworkCollector) OnGraftReceived(count int) {
	nc.gossipSubReceivedGraftCount.Add(float64(count))
}

// OnPruneReceived tracks the number of PRUNE messages received by the node from other nodes.
// PRUNE is a control message of GossipSub protocol that disconnects two nodes over a topic.
func (nc *NetworkCollector) OnPruneReceived(count int) {
	nc.gossipSubReceivedPruneCount.Add(float64(count))
}

// OnIncomingRpcAcceptedFully tracks the number of RPC messages received by the node that are fully accepted.
// An RPC may contain any number of control messages, i.e., IHAVE, IWANT, GRAFT, PRUNE, as well as the actual messages.
// A fully accepted RPC means that all the control messages are accepted and all the messages are accepted.
func (nc *NetworkCollector) OnIncomingRpcAcceptedFully() {
	nc.gossipSubIncomingRpcAcceptedFullyCount.Inc()
}

// OnIncomingRpcAcceptedOnlyForControlMessages tracks the number of RPC messages received by the node that are accepted
// only for the control messages, i.e., only for the included IHAVE, IWANT, GRAFT, PRUNE. However, the actual messages
// included in the RPC are not accepted.
// This happens mostly when the validation pipeline of GossipSub is throttled, and cannot accept more actual messages for
// validation.
func (nc *NetworkCollector) OnIncomingRpcAcceptedOnlyForControlMessages() {
	nc.gossipSubIncomingRpcAcceptedOnlyControlCount.Inc()
}

// OnIncomingRpcRejected tracks the number of RPC messages received by the node that are rejected.
// This happens mostly when the RPC is coming from a low-scored peer based on the peer scoring module of GossipSub.
func (nc *NetworkCollector) OnIncomingRpcRejected() {
	nc.gossipSubIncomingRpcRejectedCount.Inc()
}
