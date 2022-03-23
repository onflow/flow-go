package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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

	prefix string
}

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
