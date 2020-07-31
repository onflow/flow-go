package libp2p

import (
	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/control"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

var _ connmgr.ConnectionGater = (*ConnGator)(nil)

type ConnGator struct {

}

func(c *ConnGator) InterceptPeerDial(p peer.ID) bool {
	return true
}

func(c *ConnGator) InterceptAddrDial(_ peer.ID, _ ma.Multiaddr) bool {
	return true
}

func(c *ConnGator) InterceptAccept(_ network.ConnMultiaddrs) bool {
	return true
}

func(c *ConnGator) InterceptSecured(_ network.Direction, _ peer.ID, _ network.ConnMultiaddrs) bool {
	panic("not implemented")
}

func(c *ConnGator) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	panic("not implemented")
}
