package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// StreamFactory is a wrapper around libp2p host.Host to provide abstraction and encapsulation for unicast stream manager so that
// it can create libp2p streams with finer granularity.
type StreamFactory interface {
	SetStreamHandler(protocol.ID, network.StreamHandler)
	// Connect connects host to peer with peerAddrInfo.
	// All errors returned from this function can be considered benign. We expect the following errors during normal operations:
	//   - ErrSecurityProtocolNegotiationFailed this indicates there was an issue upgrading the connection.
	//   - ErrGaterDisallowedConnection this indicates the connection was disallowed by the gater.
	//   - There may be other unexpected errors from libp2p but they should be considered benign.
	Connect(context.Context, peer.AddrInfo) error
	// NewStream creates a new stream on the libp2p host.
	// Expected errors during normal operations:
	//   - ErrProtocolNotSupported this indicates remote node is running on a different spork.
	NewStream(context.Context, peer.ID, ...protocol.ID) (network.Stream, error)
}
