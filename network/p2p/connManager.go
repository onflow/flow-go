package p2p

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
)

// ConnManager provides an implementation of Libp2p's ConnManager interface (https://godoc.org/github.com/libp2p/go-libp2p-core/connmgr#ConnManager)
// It is called back by libp2p when certain events occur such as opening/closing a stream, opening/closing connection etc.
// This implementation updates networking metrics when a peer connection is added or removed
type ConnManager struct {
	connmgr.NullConnMgr                       // a null conn mgr provided by libp2p to allow implementing only the functions needed
	n                   network.Notifiee      // the notifiee callback provided by libp2p
	log                 zerolog.Logger        // logger to log connection, stream and other statistics about libp2p
	metrics             module.NetworkMetrics // metrics to report connection statistics

	plk       sync.RWMutex
	protected map[peer.ID]map[string]struct{}
}

func NewConnManager(log zerolog.Logger, metrics module.NetworkMetrics) *ConnManager {
	cn := &ConnManager{
		log:         log,
		NullConnMgr: connmgr.NullConnMgr{},
		metrics:     metrics,
		protected:   make(map[peer.ID]map[string]struct{}),
	}
	n := &network.NotifyBundle{ListenCloseF: cn.ListenCloseNotifee,
		ListenF:       cn.ListenNotifee,
		ConnectedF:    cn.Connected,
		DisconnectedF: cn.Disconnected}
	cn.n = n

	return cn
}

func (c *ConnManager) Notifee() network.Notifiee {
	return c.n
}

// ListenNotifee is called by libp2p when network starts listening on an addr
func (c *ConnManager) ListenNotifee(n network.Network, m multiaddr.Multiaddr) {
	c.log.Debug().Str("multiaddress", m.String()).Msg("listen started")
}

// called by libp2p when network stops listening on an addr
// * This is never called back by libp2p currently and may be a bug on their side
func (c *ConnManager) ListenCloseNotifee(n network.Network, m multiaddr.Multiaddr) {
	// just log the multiaddress  on which we listen
	c.log.Debug().Str("multiaddress", m.String()).Msg("listen stopped ")
}

// Connected is called by libp2p when a connection opened
func (c *ConnManager) Connected(n network.Network, con network.Conn) {
	c.logConnectionUpdate(n, con, "connection established")
	c.updateConnectionMetric(n)
}

// Disconnected is called by libp2p when a connection closed
func (c *ConnManager) Disconnected(n network.Network, con network.Conn) {
	c.logConnectionUpdate(n, con, "connection removed")
	c.updateConnectionMetric(n)
}

func (c *ConnManager) updateConnectionMetric(n network.Network) {
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

	c.metrics.InboundConnections(totalInbound)
	c.metrics.OutboundConnections(totalOutbound)
}

func (c *ConnManager) logConnectionUpdate(n network.Network, con network.Conn, logMsg string) {
	c.log.Debug().
		Str("remote_peer", con.RemotePeer().String()).
		Str("remote_addrs", con.RemoteMultiaddr().String()).
		Str("local_peer", con.LocalPeer().String()).
		Str("local_addrs", con.LocalMultiaddr().String()).
		Str("direction", con.Stat().Direction.String()).
		Int("total_connections", len(n.Conns())).
		Msg(logMsg)
}

func (cm *ConnManager) Protect(id peer.ID, tag string) {
	cm.plk.Lock()
	defer cm.plk.Unlock()

	tags, ok := cm.protected[id]
	if !ok {
		tags = make(map[string]struct{}, 2)
		cm.protected[id] = tags
	}
	tags[tag] = struct{}{}
}

func (cm *ConnManager) Unprotect(id peer.ID, tag string) (protected bool) {
	cm.plk.Lock()
	defer cm.plk.Unlock()

	tags, ok := cm.protected[id]
	if !ok {
		return false
	}
	if delete(tags, tag); len(tags) == 0 {
		delete(cm.protected, id)
		return false
	}
	return true
}

func (cm *ConnManager) IsProtected(id peer.ID, tag string) (protected bool) {
	cm.plk.RLock()
	defer cm.plk.RUnlock()

	tags, ok := cm.protected[id]
	if !ok {
		return false
	}

	if tag == "" {
		return true
	}

	_, protected = tags[tag]
	return protected
}
