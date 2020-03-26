package libp2p

import (
	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
)

// ConnManager provides an implementation of Libp2p's ConnManager interface (https://godoc.org/github.com/libp2p/go-libp2p-core/connmgr#ConnManager)
// It is called back by libp2p when certain events occur such as opening/closing a stream, opening/closing connection etc.
// This implementation only logs the call back for debugging purposes.
type ConnManager struct {
	connmgr.NullConnMgr                  // a null conn mgr provided by libp2p to allow implementing only the functions needed
	n                   network.Notifiee // the notifiee callback provided by libp2p
	log                 zerolog.Logger   // logger to log connection, stream and other statistics about libp2p
}

func NewConnManager(log zerolog.Logger) ConnManager {
	cn := ConnManager{
		log:         log,
		NullConnMgr: connmgr.NullConnMgr{},
	}
	n := &network.NotifyBundle{ListenCloseF: cn.ListenCloseNotifee, ListenF: cn.ListenNotifee, DisconnectedF: cn.Disconnected}
	cn.n = n
	return cn
}

func (c ConnManager) Notifee() network.Notifiee {
	return c.n
}

// called by libp2p when network starts listening on an addr
func (c ConnManager) ListenNotifee(n network.Network, m multiaddr.Multiaddr) {
	c.log.Debug().Str("multiaddress", m.String()).Msg("listen started")
}

// called by libp2p when network stops listening on an addr
// * This is never called back by libp2p currently and may be a bug on their side
func (c ConnManager) ListenCloseNotifee(n network.Network, m multiaddr.Multiaddr) {
	// just log the multiaddress  on which we listen
	c.log.Debug().Str("multiaddress", m.String()).Msg("listen stopped ")
}

// called by libp2p when a connection opened
func (c ConnManager) Connected(n network.Network, con network.Conn) {
	c.log.Debug().Str("remote_peer", con.RemotePeer().String()).Int("total_conns", len(n.Conns())).Msg("opened connection")
}

// called by libp2p when a connection closed
func (c ConnManager) Disconnected(n network.Network, con network.Conn) {
	c.log.Debug().Str("remote_peer", con.RemotePeer().String()).Int("total_conns", len(n.Conns())).Msg("closed connection")
}
