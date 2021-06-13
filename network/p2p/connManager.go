package p2p

import (
	"context"
	"time"

	libp2pcore "github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
)

var _ libp2pcore.ConnManager = &ConnManager{}

// ConnManager provides an implementation of Libp2p's ConnManager interface (https://godoc.org/github.com/libp2p/go-libp2p-core/connmgr#ConnManager)
// It is called back by libp2p when certain events occur such as opening/closing a stream, opening/closing connection etc.
// It is a wrapper around Libp2p's BasicConnMgr with the addition of updates to networking metrics when a peer connection is added or removed
type ConnManager struct {
	basicConnMgr *connmgr.BasicConnMgr        // the basic conn mgr provided by libp2p
	n                   network.Notifiee      // the notifiee callback provided by libp2p
	log                 zerolog.Logger        // logger to log connection, stream and other statistics about libp2p
	metrics             module.NetworkMetrics // metrics to report connection statistics
}

func NewConnManager(low int, high int, log zerolog.Logger, metrics module.NetworkMetrics) ConnManager {

	// do not support decay
	neverDecay := connmgr.DecayerConfig(&connmgr.DecayerCfg{
		Resolution: time.Hour,
	})
	basicConnMgr := connmgr.NewConnManager(low, high, 10 * time.Second, neverDecay)

	cn := ConnManager{
		log:         log,
		basicConnMgr: basicConnMgr,
		metrics:     metrics,
	}
	n := &network.NotifyBundle{ListenCloseF: cn.ListenCloseNotifee,
		ListenF:       cn.ListenNotifee,
		ConnectedF:    cn.Connected,
		DisconnectedF: cn.Disconnected}
	cn.n = n
	return cn
}

func (c  *ConnManager) Notifee() network.Notifiee {
	return c.n
}

// called by libp2p when network starts listening on an addr
func (c  *ConnManager) ListenNotifee(n network.Network, m multiaddr.Multiaddr) {
	c.basicConnMgr.Notifee().Listen(n, m)
	c.log.Debug().Str("multiaddress", m.String()).Msg("listen started")
}

// called by libp2p when network stops listening on an addr
// * This is never called back by libp2p currently and may be a bug on their side
func (c  *ConnManager) ListenCloseNotifee(n network.Network, m multiaddr.Multiaddr) {
	c.basicConnMgr.Notifee().ListenClose(n, m)
	// just log the multiaddress  on which we listen
	c.log.Debug().Str("multiaddress", m.String()).Msg("listen stopped ")
}

// called by libp2p when a connection opened
func (c  *ConnManager) Connected(n network.Network, con network.Conn) {
	c.basicConnMgr.Notifee().Connected(n, con)
	c.logConnectionUpdate(n, con, "connection established")
	c.updateConnectionMetric(n)
}

// called by libp2p when a connection closed
func (c  *ConnManager) Disconnected(n network.Network, con network.Conn) {
	c.basicConnMgr.Notifee().Disconnected(n, con)
	c.logConnectionUpdate(n, con, "connection removed")
	c.updateConnectionMetric(n)
}

func (c  *ConnManager) updateConnectionMetric(n network.Network) {
	var inbound uint = 0
	var outbound uint = 0
	for _, conn := range n.Conns() {
		switch conn.Stat().Direction {
		case network.DirInbound:
			inbound++
		case network.DirOutbound:
			outbound++
		}
	}

	c.metrics.InboundConnections(inbound)
	c.metrics.OutboundConnections(outbound)
}

func (c  *ConnManager) logConnectionUpdate(n network.Network, con network.Conn, logMsg string) {
	c.log.Debug().
		Str("remote_peer", con.RemotePeer().String()).
		Str("remote_addrs", con.RemoteMultiaddr().String()).
		Str("local_peer", con.LocalPeer().String()).
		Str("local_addrs", con.LocalMultiaddr().String()).
		Str("direction", con.Stat().Direction.String()).
		Int("total_connections", len(n.Conns())).
		Msg(logMsg)
}

func (c *ConnManager) TagPeer(id peer.ID, s string, i int) {
	panic("implement me")
}

func (c *ConnManager) UntagPeer(p peer.ID, tag string) {
	panic("implement me")
}

func (c *ConnManager) UpsertTag(p peer.ID, tag string, upsert func(int) int) {
	panic("implement me")
}

func (c *ConnManager) GetTagInfo(p peer.ID) *libp2pcore.TagInfo {
	panic("implement me")
}

func (c *ConnManager) TrimOpenConns(ctx context.Context) {
	panic("implement me")
}

func (c *ConnManager) Protect(id peer.ID, tag string) {
	panic("implement me")
}

func (c *ConnManager) Unprotect(id peer.ID, tag string) (protected bool) {
	panic("implement me")
}

func (c *ConnManager) IsProtected(id peer.ID, tag string) (protected bool) {
	panic("implement me")
}

func (c *ConnManager) Close() error {
	panic("implement me")
}
