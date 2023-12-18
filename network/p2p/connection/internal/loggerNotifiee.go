package internal

import (
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p/logging"
)

type LoggerNotifiee struct {
	logger  zerolog.Logger
	metrics module.LibP2PConnectionMetrics
}

var _ network.Notifiee = (*LoggerNotifiee)(nil)

func NewLoggerNotifiee(logger zerolog.Logger, metrics module.LibP2PConnectionMetrics) *LoggerNotifiee {
	return &LoggerNotifiee{
		logger:  logger,
		metrics: metrics,
	}
}

func (l *LoggerNotifiee) Listen(_ network.Network, multiaddr multiaddr.Multiaddr) {
	// just log the multiaddress on which we listen
	l.logger.Debug().Str("multiaddress", multiaddr.String()).Msg("listen started")
}

func (l *LoggerNotifiee) ListenClose(_ network.Network, multiaddr multiaddr.Multiaddr) {
	l.logger.Debug().Str("multiaddress", multiaddr.String()).Msg("listen stopped")
}

func (l *LoggerNotifiee) Connected(n network.Network, conn network.Conn) {
	l.updateConnectionMetric(n)
	lg := l.connectionUpdateLogger(n, conn)
	lg.Debug().Msg("connection established")
}

func (l *LoggerNotifiee) Disconnected(n network.Network, conn network.Conn) {
	l.updateConnectionMetric(n)
	lg := l.connectionUpdateLogger(n, conn)
	lg.Debug().Msg("connection closed")
}

func (l *LoggerNotifiee) connectionUpdateLogger(n network.Network, con network.Conn) zerolog.Logger {
	return l.logger.With().
		Str("remote_peer", p2plogging.PeerId(con.RemotePeer())).
		Str("remote_address", con.RemoteMultiaddr().String()).
		Str("local_peer", p2plogging.PeerId(con.LocalPeer())).
		Str("local_address", con.LocalMultiaddr().String()).
		Str("direction", con.Stat().Direction.String()).
		Int("total_connections", len(n.Conns())).Logger()
}

func (l *LoggerNotifiee) updateConnectionMetric(n network.Network) {
	var totalInbound uint = 0
	var totalOutbound uint = 0

	for _, conn := range n.Conns() {
		switch conn.Stat().Direction {
		case network.DirInbound:
			totalInbound++
		case network.DirOutbound:
			totalOutbound++
		}
	}

	l.metrics.InboundConnections(totalInbound)
	l.metrics.OutboundConnections(totalOutbound)
}
