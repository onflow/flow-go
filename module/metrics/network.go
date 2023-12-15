package metrics

import (
	"strconv"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	logging2 "github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	_   = iota
	KiB = 1 << (10 * iota)
	MiB
	GiB
)

type NetworkCollector struct {
	*UnicastManagerMetrics
	*LibP2PResourceManagerMetrics
	*GossipSubMetrics
	*GossipSubScoreMetrics
	*GossipSubLocalMeshMetrics
	*GossipSubRpcValidationInspectorMetrics
	*AlspMetrics
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

	// security metrics
	unAuthorizedMessagesCount       *prometheus.CounterVec
	rateLimitedUnicastMessagesCount *prometheus.CounterVec
	violationReportSkippedCount     prometheus.Counter

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

func NewNetworkCollector(logger zerolog.Logger, opts ...NetworkCollectorOpt) *NetworkCollector {
	nc := &NetworkCollector{}

	for _, opt := range opts {
		opt(nc)
	}

	nc.UnicastManagerMetrics = NewUnicastManagerMetrics(nc.prefix)
	nc.LibP2PResourceManagerMetrics = NewLibP2PResourceManagerMetrics(logger, nc.prefix)
	nc.GossipSubLocalMeshMetrics = NewGossipSubLocalMeshMetrics(nc.prefix)
	nc.GossipSubMetrics = NewGossipSubMetrics(nc.prefix)
	nc.GossipSubScoreMetrics = NewGossipSubScoreMetrics(nc.prefix)
	nc.GossipSubRpcValidationInspectorMetrics = NewGossipSubRPCValidationInspectorMetrics(nc.prefix)
	nc.AlspMetrics = NewAlspMetrics()

	nc.outboundMessageSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "outbound_message_size_bytes",
			Help:      "size of the outbound network message",
			Buckets:   []float64{KiB, 100 * KiB, 1 * MiB},
		}, []string{LabelChannel, LabelProtocol, LabelMessage},
	)

	nc.inboundMessageSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "inbound_message_size_bytes",
			Help:      "size of the inbound network message",
			Buckets:   []float64{KiB, 100 * KiB, 1 * MiB},
		}, []string{LabelChannel, LabelProtocol, LabelMessage},
	)

	nc.duplicateMessagesDropped = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      nc.prefix + "duplicate_messages_dropped",
			Help:      "number of duplicate messages dropped",
		}, []string{LabelChannel, LabelProtocol, LabelMessage},
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

	nc.violationReportSkippedCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemSecurity,
			Name:      nc.prefix + "slashing_violation_reports_skipped_count",
			Help:      "number of slashing violations consumer violations that were not reported for misbehavior because the identity of the sender not known",
		},
	)

	return nc
}

// OutboundMessageSent collects metrics related to a message sent by the node.
func (nc *NetworkCollector) OutboundMessageSent(sizeBytes int, topic, protocol, messageType string) {
	nc.outboundMessageSize.WithLabelValues(topic, protocol, messageType).Observe(float64(sizeBytes))
}

// InboundMessageReceived collects metrics related to a message received by the node.
func (nc *NetworkCollector) InboundMessageReceived(sizeBytes int, topic, protocol, messageType string) {
	nc.inboundMessageSize.WithLabelValues(topic, protocol, messageType).Observe(float64(sizeBytes))
}

// DuplicateInboundMessagesDropped increments the metric tracking the number of duplicate messages dropped by the node.
func (nc *NetworkCollector) DuplicateInboundMessagesDropped(topic, protocol, messageType string) {
	nc.duplicateMessagesDropped.WithLabelValues(topic, protocol, messageType).Add(1)
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

// MessageProcessingStarted increments the metric tracking the number of messages being processed by the node.
func (nc *NetworkCollector) MessageProcessingStarted(topic string) {
	nc.numMessagesProcessing.WithLabelValues(topic).Inc()
}

// UnicastMessageSendingStarted increments the metric tracking the number of unicast messages sent by the node.
func (nc *NetworkCollector) UnicastMessageSendingStarted(topic string) {
	nc.numDirectMessagesSending.WithLabelValues(topic).Inc()
}

// UnicastMessageSendingCompleted decrements the metric tracking the number of unicast messages sent by the node.
func (nc *NetworkCollector) UnicastMessageSendingCompleted(topic string) {
	nc.numDirectMessagesSending.WithLabelValues(topic).Dec()
}

func (nc *NetworkCollector) RoutingTablePeerAdded() {
	nc.routingTableSize.Inc()
}

func (nc *NetworkCollector) RoutingTablePeerRemoved() {
	nc.routingTableSize.Dec()
}

// MessageProcessingFinished tracks the time spent by the node to process a message and decrements the metric tracking
// the number of messages being processed by the node.
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

// OnRateLimitedPeer tracks the number of rate limited messages seen on the network.
func (nc *NetworkCollector) OnRateLimitedPeer(peerID peer.ID, role, msgType, topic, reason string) {
	nc.logger.Warn().
		Str("peer_id", logging2.PeerId(peerID)).
		Str("role", role).
		Str("message_type", msgType).
		Str("topic", topic).
		Str("reason", reason).
		Bool(logging.KeySuspicious, true).
		Msg("unicast peer rate limited")
	nc.rateLimitedUnicastMessagesCount.WithLabelValues(role, msgType, topic, reason).Inc()
}

// OnViolationReportSkipped tracks the number of slashing violations consumer violations that were not
// reported for misbehavior when the identity of the sender not known.
func (nc *NetworkCollector) OnViolationReportSkipped() {
	nc.violationReportSkippedCount.Inc()
}
