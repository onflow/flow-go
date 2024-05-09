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

	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/utils/logging"
)

type LibP2PResourceManagerMetrics struct {
	logger zerolog.Logger
	// libp2p resource manager metrics
	// connections
	allowConnectionCount *prometheus.CounterVec
	blockConnectionCount *prometheus.CounterVec
	// streams
	allowStreamCount *prometheus.CounterVec
	blockStreamCount *prometheus.CounterVec
	// peers
	allowPeerCount prometheus.Counter
	blockPeerCount prometheus.Counter
	// protocol
	allowProtocolCount     prometheus.Counter
	blockProtocolCount     prometheus.Counter
	blockProtocolPeerCount prometheus.Counter
	// services
	allowServiceCount     prometheus.Counter
	blockServiceCount     prometheus.Counter
	blockServicePeerCount prometheus.Counter
	// memory
	allowMemoryHistogram prometheus.Histogram
	blockMemoryHistogram prometheus.Histogram

	prefix string
}

var _ rcmgr.MetricsReporter = (*LibP2PResourceManagerMetrics)(nil)

func NewLibP2PResourceManagerMetrics(logger zerolog.Logger, prefix string) *LibP2PResourceManagerMetrics {
	l := &LibP2PResourceManagerMetrics{logger: logger, prefix: prefix}

	l.allowConnectionCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_allow_connection_total",
		Help:      "total number of connections allowed by the libp2p resource manager",

		// labels are: "inbound", "outbound" and whether the connection is using a file descriptor
	}, []string{LabelConnectionDirection, LabelConnectionUseFD})

	l.blockConnectionCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_block_connection_total",
		Help:      "total number of connections blocked by the libp2p resource manager",

		// labels are: "inbound", "outbound" and whether the connection is using a file descriptor
	}, []string{LabelConnectionDirection, LabelConnectionUseFD})

	l.allowStreamCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_allow_stream_total",
		Help:      "total number of streams allowed by the libp2p resource manager",
	}, []string{LabelConnectionDirection})

	l.blockStreamCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_block_stream_total",
		Help:      "total number of streams blocked by the libp2p resource manager",
	}, []string{LabelConnectionDirection})

	l.allowPeerCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_allow_peer_total",
		Help:      "total number of remote peers allowed by the libp2p resource manager to attach to their relevant incoming/outgoing streams",
	})

	// Note: this is a low level metric than blockProtocolPeerCount.
	// This metric is incremented when a peer is blocked by the libp2p resource manager on attaching as one end of a stream (on any protocol).
	l.blockPeerCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_block_peer_total",
		Help:      "total number of remote peers blocked by the libp2p resource manager from attaching to their relevant incoming/outgoing streams",
	})

	l.allowProtocolCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_allow_protocol_total",
		Help:      "total number of protocols allowed by the libp2p resource manager to attach to their relevant incoming/outgoing streams",
	})

	l.blockProtocolCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_block_protocol_total",
		Help:      "total number of protocols blocked by the libp2p resource manager from attaching to their relevant incoming/outgoing streams",
	})

	// Note: this is a higher level metric than blockPeerCount and blockProtocolCount.
	// This metric is incremented when a peer is already attached as one end of a stream but on a different reserved protocol.
	l.blockProtocolPeerCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_block_protocol_peer_total",
		Help:      "total number of remote peers blocked by the libp2p resource manager from attaching to their relevant incoming/outgoing streams on a specific protocol",
	})

	l.allowServiceCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_allow_service_total",
		Help:      "total number of remote services (e.g., ping, relay) allowed by the libp2p resource manager to attach to their relevant incoming/outgoing streams",
	})

	l.blockServiceCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_block_service_total",
		Help:      "total number of remote services (e.g., ping, relay) blocked by the libp2p resource manager from attaching to their relevant incoming/outgoing streams",
	})

	// Note: this is a higher level metric than blockServiceCount and blockPeerCount.
	// This metric is incremented when a service is already attached as one end of a stream but on a different reserved protocol.
	l.blockServicePeerCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_block_service_peer_total",
		Help:      "total number of remote services (e.g., ping, relay) blocked by the libp2p resource manager from attaching to their relevant incoming/outgoing streams on a specific peer",
	})

	l.allowMemoryHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_allowed_memory_bytes",
		Help:      "size of memory allocation requests allowed by the libp2p resource manager",
		Buckets:   []float64{KiB, 10 * KiB, 100 * KiB, 500 * KiB, 1 * MiB, 10 * MiB, 100 * MiB, 500 * MiB, 1 * GiB},
	})

	l.blockMemoryHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemLibp2p,
		Name:      l.prefix + "resource_manager_blocked_memory_bytes",
		Help:      "size of memory allocation requests blocked by the libp2p resource manager",
		Buckets:   []float64{KiB, 10 * KiB, 100 * KiB, 500 * KiB, 1 * MiB, 10 * MiB, 100 * MiB, 500 * MiB, 1 * GiB},
	})

	return l
}

func (l *LibP2PResourceManagerMetrics) AllowConn(dir network.Direction, usefd bool) {
	l.allowConnectionCount.WithLabelValues(dir.String(), strconv.FormatBool(usefd)).Inc()
	l.logger.Trace().Str("direction", dir.String()).Bool("use_file_descriptor", usefd).Msg("allowing connection")
}

func (l *LibP2PResourceManagerMetrics) BlockConn(dir network.Direction, usefd bool) {
	l.blockConnectionCount.WithLabelValues(dir.String(), strconv.FormatBool(usefd)).Inc()
	l.logger.Debug().Bool(logging.KeySuspicious, true).Str("direction", dir.String()).Bool("using_file_descriptor", usefd).Msg("blocking connection")
}

func (l *LibP2PResourceManagerMetrics) AllowStream(p peer.ID, dir network.Direction) {
	l.allowStreamCount.WithLabelValues(dir.String()).Inc()
	l.logger.Trace().Str("peer", p2plogging.PeerId(p)).Str("direction", dir.String()).Msg("allowing stream")
}

func (l *LibP2PResourceManagerMetrics) BlockStream(p peer.ID, dir network.Direction) {
	l.blockStreamCount.WithLabelValues(dir.String()).Inc()
	l.logger.Debug().Bool(logging.KeySuspicious, true).Str("peer", p2plogging.PeerId(p)).Str("direction", dir.String()).Msg("blocking stream")
}

func (l *LibP2PResourceManagerMetrics) AllowPeer(p peer.ID) {
	l.allowPeerCount.Inc()
	l.logger.Trace().Str("peer", p2plogging.PeerId(p)).Msg("allowing peer")
}

func (l *LibP2PResourceManagerMetrics) BlockPeer(p peer.ID) {
	l.blockPeerCount.Inc()
	l.logger.Debug().Bool(logging.KeySuspicious, true).Str("peer", p2plogging.PeerId(p)).Msg("blocking peer")
}

func (l *LibP2PResourceManagerMetrics) AllowProtocol(proto protocol.ID) {
	l.allowProtocolCount.Inc()
	l.logger.Trace().Str("protocol", string(proto)).Msg("allowing protocol")
}

func (l *LibP2PResourceManagerMetrics) BlockProtocol(proto protocol.ID) {
	l.blockProtocolCount.Inc()
	l.logger.Debug().Bool(logging.KeySuspicious, true).Str("protocol", string(proto)).Msg("blocking protocol")
}

func (l *LibP2PResourceManagerMetrics) BlockProtocolPeer(proto protocol.ID, p peer.ID) {
	l.blockProtocolPeerCount.Inc()
	l.logger.Debug().Bool(logging.KeySuspicious, true).Str("protocol", string(proto)).Str("peer", p2plogging.PeerId(p)).Msg("blocking protocol for peer")
}

func (l *LibP2PResourceManagerMetrics) AllowService(svc string) {
	l.allowServiceCount.Inc()
	l.logger.Trace().Str("service", svc).Msg("allowing service")
}

func (l *LibP2PResourceManagerMetrics) BlockService(svc string) {
	l.blockServiceCount.Inc()
	l.logger.Debug().Bool(logging.KeySuspicious, true).Str("service", svc).Msg("blocking service")
}

func (l *LibP2PResourceManagerMetrics) BlockServicePeer(svc string, p peer.ID) {
	l.blockServicePeerCount.Inc()
	l.logger.Debug().Bool(logging.KeySuspicious, true).Str("service", svc).Str("peer", p2plogging.PeerId(p)).Msg("blocking service for peer")
}

func (l *LibP2PResourceManagerMetrics) AllowMemory(size int) {
	l.allowMemoryHistogram.Observe(float64(size))
	l.logger.Trace().Int("size", size).Msg("allowing memory")
}

func (l *LibP2PResourceManagerMetrics) BlockMemory(size int) {
	l.blockMemoryHistogram.Observe(float64(size))
	l.logger.Debug().Bool(logging.KeySuspicious, true).Int("size", size).Msg("blocking memory")
}
