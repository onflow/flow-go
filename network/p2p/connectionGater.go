package p2p

import (
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type ConnectionGater interface {
	InterceptPeerDial(p peer.ID) (allow bool)

	InterceptAddrDial(peer.ID, multiaddr.Multiaddr) (allow bool)

	InterceptAccept(network.ConnMultiaddrs) (allow bool)

	InterceptSecured(network.Direction, peer.ID, network.ConnMultiaddrs) (allow bool)

	InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason)
}
