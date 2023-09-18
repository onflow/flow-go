package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
)

// StreamFactory is a wrapper around libp2p host.Host to provide abstraction and encapsulation for unicast stream manager so that
// it can create libp2p streams with finer granularity.
type StreamFactory interface {
	SetStreamHandler(protocol.ID, network.StreamHandler)
	DialAddress(peer.ID) []multiaddr.Multiaddr
	ClearBackoff(peer.ID)
	// Connect connects host to peer with peerID.
	// Expected errors during normal operations:
	//   - NewSecurityProtocolNegotiationErr this indicates there was an issue upgrading the connection.
	Connect(context.Context, peer.AddrInfo) error
	// NewStream creates a new stream on the libp2p host.
	// Expected errors during normal operations:
	//   - ErrProtocolNotSupported this indicates remote node is running on a different spork.
	NewStream(context.Context, peer.ID, ...protocol.ID) (network.Stream, error)
}
