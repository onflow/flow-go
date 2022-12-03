package metrics

import (
	"strconv"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
)

type LibP2PResourceManagerMetrics struct {
	logger zerolog.Logger
	// libp2p resource manager metrics
	// connections
	libp2pAllowConnectionCount *prometheus.CounterVec
	libp2pBlockConnectionCount *prometheus.CounterVec
	// streams
	libp2pAllowStreamCount *prometheus.CounterVec
	libp2pBlockStreamCount *prometheus.CounterVec
	// peers
	libp2pAllowPeerCount prometheus.Counter
	libp2pBlockPeerCount prometheus.Counter
	// protocol
	libp2pAllowProtocolCount     prometheus.Counter
	libp2pBlockProtocolCount     prometheus.Counter
	libp2pBlockProtocolPeerCount prometheus.Counter
	// services
	libp2pAllowServiceCount     prometheus.Counter
	libp2pBlockServiceCount     prometheus.Counter
	libp2pBlockServicePeerCount prometheus.Counter
	// memory
	libp2pAllowMemoryHistogram prometheus.Histogram
	libp2pBlockMemoryHistogram prometheus.Histogram

	prefix string
}

var _ rcmgr.MetricsReporter = (*LibP2PResourceManagerMetrics)(nil)

func NewLibP2PResourceManagerMetrics(logger zerolog.Logger, prefix string) *LibP2PResourceManagerMetrics {
	l := &LibP2PResourceManagerMetrics{logger: logger, prefix: prefix}

	l.libp2pAllowConnectionCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_allow_connection_total",
			Help:      "total number of connections allowed by the libp2p resource manager",

			// labels are: "inbound", "outbound" and whether the connection is using a file descriptor
		}, []string{LabelConnectionDirection, LabelConnectionUseFD},
	)

	l.libp2pBlockConnectionCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_block_connection_total",
			Help:      "total number of connections blocked by the libp2p resource manager",

			// labels are: "inbound", "outbound" and whether the connection is using a file descriptor
		}, []string{LabelConnectionDirection, LabelConnectionUseFD},
	)

	l.libp2pAllowStreamCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_allow_stream_total",
			Help:      "total number of streams allowed by the libp2p resource manager",
		}, []string{LabelConnectionDirection},
	)

	l.libp2pBlockStreamCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_block_stream_total",
			Help:      "total number of streams blocked by the libp2p resource manager",
		}, []string{LabelConnectionDirection},
	)

	l.libp2pAllowPeerCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_allow_peer_total",
			Help:      "total number of remote peers allowed by the libp2p resource manager to attach to their relevant incoming/outgoing streams",
		},
	)

	// Note: this is a low level metric than libp2pBlockProtocolPeerCount.
	// This metric is incremented when a peer is blocked by the libp2p resource manager on attaching as one end of a stream (on any protocol).
	l.libp2pBlockPeerCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_block_peer_total",
			Help:      "total number of remote peers blocked by the libp2p resource manager from attaching to their relevant incoming/outgoing streams",
		},
	)

	l.libp2pAllowProtocolCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_allow_protocol_total",
			Help:      "total number of protocols allowed by the libp2p resource manager to attach to their relevant incoming/outgoing streams",
		},
	)

	l.libp2pBlockProtocolCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_block_protocol_total",
			Help:      "total number of protocols blocked by the libp2p resource manager from attaching to their relevant incoming/outgoing streams",
		},
	)

	// Note: this is a higher level metric than libp2pBlockPeerCount and libp2pBlockProtocolCount.
	// This metric is incremented when a peer is already attached as one end of a stream but on a different reserved protocol.
	l.libp2pBlockProtocolPeerCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_block_protocol_peer_total",
			Help:      "total number of remote peers blocked by the libp2p resource manager from attaching to their relevant incoming/outgoing streams on a specific protocol",
		},
	)

	l.libp2pAllowServiceCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_allow_service_total",
			Help:      "total number of remote services (e.g., ping, relay) allowed by the libp2p resource manager to attach to their relevant incoming/outgoing streams",
		},
	)

	l.libp2pBlockServiceCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_block_service_total",
			Help:      "total number of remote services (e.g., ping, relay) blocked by the libp2p resource manager from attaching to their relevant incoming/outgoing streams",
		},
	)

	// Note: this is a higher level metric than libp2pBlockServiceCount and libp2pBlockPeerCount.
	// This metric is incremented when a service is already attached as one end of a stream but on a different reserved protocol.
	l.libp2pBlockServicePeerCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_block_service_peer_total",
			Help:      "total number of remote services (e.g., ping, relay) blocked by the libp2p resource manager from attaching to their relevant incoming/outgoing streams on a specific peer",
		},
	)

	l.libp2pAllowMemoryHistogram = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_allowed_memory_bytes",
			Help:      "size of memory allocation requests allowed by the libp2p resource manager",
			Buckets:   []float64{KiB, 10 * KiB, 100 * KiB, 500 * KiB, 1 * MiB, 10 * MiB, 100 * MiB, 500 * MiB, 1 * GiB},
		},
	)

	l.libp2pBlockMemoryHistogram = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemLibp2p,
			Name:      l.prefix + "resource_manager_blocked_memory_bytes",
			Help:      "size of memory allocation requests blocked by the libp2p resource manager",
			Buckets:   []float64{KiB, 10 * KiB, 100 * KiB, 500 * KiB, 1 * MiB, 10 * MiB, 100 * MiB, 500 * MiB, 1 * GiB},
		},
	)

	return l
}

func (l *LibP2PResourceManagerMetrics) AllowConn(dir network.Direction, usefd bool) {
	l.libp2pAllowConnectionCount.WithLabelValues(dir.String(), strconv.FormatBool(usefd)).Inc()
	l.logger.Trace().Str("direction", dir.String()).Bool("use_file_descriptor", usefd).Msg("allowing connection")
}

func (l *LibP2PResourceManagerMetrics) BlockConn(dir network.Direction, usefd bool) {
	l.libp2pBlockConnectionCount.WithLabelValues(dir.String(), strconv.FormatBool(usefd)).Inc()
	l.logger.Warn().Str("direction", dir.String()).Bool("using_file_descriptor", usefd).Msg("blocking connection")
}

func (l *LibP2PResourceManagerMetrics) AllowStream(p peer.ID, dir network.Direction) {
	l.libp2pAllowStreamCount.WithLabelValues(dir.String()).Inc()
	l.logger.Trace().Str("peer", p.String()).Str("direction", dir.String()).Msg("allowing stream")
}

func (l *LibP2PResourceManagerMetrics) BlockStream(p peer.ID, dir network.Direction) {
	l.libp2pBlockStreamCount.WithLabelValues(dir.String()).Inc()
	l.logger.Warn().Str("peer", p.String()).Str("direction", dir.String()).Msg("blocking stream")
}

func (l *LibP2PResourceManagerMetrics) AllowPeer(p peer.ID) {
	l.libp2pAllowPeerCount.Inc()
	l.logger.Trace().Str("peer", p.String()).Msg("allowing peer")
}

func (l *LibP2PResourceManagerMetrics) BlockPeer(p peer.ID) {
	l.libp2pBlockPeerCount.Inc()
	l.logger.Warn().Str("peer", p.String()).Msg("blocking peer")
}

func (l *LibP2PResourceManagerMetrics) AllowProtocol(proto protocol.ID) {
	l.libp2pAllowProtocolCount.Inc()
	l.logger.Trace().Str("protocol", string(proto)).Msg("allowing protocol")
}

func (l *LibP2PResourceManagerMetrics) BlockProtocol(proto protocol.ID) {
	l.libp2pBlockProtocolCount.Inc()
	l.logger.Warn().Str("protocol", string(proto)).Msg("blocking protocol")
}

func (l *LibP2PResourceManagerMetrics) BlockProtocolPeer(proto protocol.ID, p peer.ID) {
	l.libp2pBlockProtocolPeerCount.Inc()
	l.logger.Warn().Str("protocol", string(proto)).Str("peer", p.String()).Msg("blocking protocol for peer")
}

func (l *LibP2PResourceManagerMetrics) AllowService(svc string) {
	l.libp2pAllowServiceCount.Inc()
	l.logger.Trace().Str("service", svc).Msg("allowing service")
}

func (l *LibP2PResourceManagerMetrics) BlockService(svc string) {
	l.libp2pBlockServiceCount.Inc()
	l.logger.Warn().Str("service", svc).Msg("blocking service")
}

func (l *LibP2PResourceManagerMetrics) BlockServicePeer(svc string, p peer.ID) {
	l.libp2pBlockServicePeerCount.Inc()
	l.logger.Warn().Str("service", svc).Str("peer", p.String()).Msg("blocking service for peer")
}

func (l *LibP2PResourceManagerMetrics) AllowMemory(size int) {
	l.libp2pAllowMemoryHistogram.Observe(float64(size))
	l.logger.Trace().Int("size", size).Msg("allowing memory")
}

func (l *LibP2PResourceManagerMetrics) BlockMemory(size int) {
	l.libp2pBlockMemoryHistogram.Observe(float64(size))
	l.logger.Warn().Int("size", size).Msg("blocking memory")
}
