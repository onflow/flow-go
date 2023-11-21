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
	// NewStream creates a new stream on the libp2p host.
	// Expected errors during normal operations:
	//   - ErrProtocolNotSupported this indicates remote node is running on a different spork.
	NewStream(context.Context, peer.ID, protocol.ID) (network.Stream, error)
}
