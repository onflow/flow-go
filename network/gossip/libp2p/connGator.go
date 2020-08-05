package libp2p

import (
	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/control"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

var _ connmgr.ConnectionGater = (*connGater)(nil)

type connGater struct {
	peerIDs map[peer.ID]struct{}
	multiAddrs map[ma.Multiaddr]struct{}
}

func newConnGater(ids []peer.ID, addrs []ma.Multiaddr) *connGater {
	peerIDs := make(map[peer.ID]struct{}, len(ids))
	for _, id := range ids {
		peerIDs[id] = struct{}{}
	}
	multiAddrs := make(map[ma.Multiaddr]struct{}, len(ids))
	for _, id := range addrs {
		multiAddrs[id] = struct{}{}
	}
	return &connGater{
		peerIDs: peerIDs,
		multiAddrs: multiAddrs,
	}
}

func(c *connGater) InterceptPeerDial(p peer.ID) bool {
	return c.validPeerID(p)
}

func(c *connGater) InterceptAddrDial(p peer.ID, _ ma.Multiaddr) bool {
	return c.validPeerID(p)
}

func(c *connGater) InterceptAccept(cm network.ConnMultiaddrs) bool {
	remoteAddr := cm.RemoteMultiaddr()
	return c.validMultiAddr(remoteAddr)
}

func(c *connGater) InterceptSecured(_ network.Direction, _ peer.ID, _ network.ConnMultiaddrs) bool {
	panic("not implemented")
}

func(c *connGater) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	panic("not implemented")
}

func(c *connGater) validPeerID(p peer.ID) bool {
	_, ok := c.peerIDs[p]
	return ok
}

func(c *connGater) validMultiAddr(addr ma.Multiaddr) bool {
	_, ok := c.multiAddrs[addr]
	return ok
}
