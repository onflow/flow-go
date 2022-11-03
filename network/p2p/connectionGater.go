package p2p

import (
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// ConnectionGater is a copy of the libp2p ConnectionGater interface:
// https://github.com/libp2p/go-libp2p/blob/master/core/connmgr/gater.go#L54
// We use it here to generate a mock for testing through testify mock.
type ConnectionGater interface {
	InterceptPeerDial(p peer.ID) (allow bool)

	InterceptAddrDial(peer.ID, multiaddr.Multiaddr) (allow bool)

	InterceptAccept(network.ConnMultiaddrs) (allow bool)

	InterceptSecured(network.Direction, peer.ID, network.ConnMultiaddrs) (allow bool)

	InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason)
}
