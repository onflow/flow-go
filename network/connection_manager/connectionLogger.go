package connection_manager

import (
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network"
)

var _ network.Observer = &ConnectionLogger{}

type ConnectionLogger struct {
	*libp2pnet.NoopNotifiee
	logger zerolog.Logger
}

func NewConnectionLogger(logger zerolog.Logger) *ConnectionLogger {
	return &ConnectionLogger{
		NoopNotifiee: libp2pnet.GlobalNoopNotifiee,
		logger:       logger,
	}
}

// called by libp2p when network starts listening on an addr
func (c *ConnectionLogger) ListenNotifee(_ libp2pnet.Network, m multiaddr.Multiaddr) {
	c.logger.Debug().Str("multiaddress", m.String()).Msg("listen started")
}

// called by libp2p when network stops listening on an addr
// * This is never called back by libp2p currently and may be a bug on their side
func (c *ConnectionLogger) ListenCloseNotifee(_ libp2pnet.Network, m multiaddr.Multiaddr) {
	// just log the multiaddress  on which we listen
	c.logger.Debug().Str("multiaddress", m.String()).Msg("listen stopped ")
}

func (c ConnectionLogger) Connected(n libp2pnet.Network, conn libp2pnet.Conn) {
	c.logConnectionUpdate(n, conn, "connection established")
}

func (c ConnectionLogger) Disconnected(n libp2pnet.Network, conn libp2pnet.Conn) {
	c.logConnectionUpdate(n, conn, "connection removed")
}

func (c *ConnectionLogger) logConnectionUpdate(n libp2pnet.Network, con libp2pnet.Conn, logMsg string) {
	c.logger.Debug().
		Str("remote_peer", con.RemotePeer().String()).
		Str("remote_addrs", con.RemoteMultiaddr().String()).
		Str("local_peer", con.LocalPeer().String()).
		Str("local_addrs", con.LocalMultiaddr().String()).
		Str("direction", con.Stat().Direction.String()).
		Int("total_connections", len(n.Conns())).
		Msg(logMsg)
}
