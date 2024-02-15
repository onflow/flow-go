package p2p

import (
	"context"

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
)

// UnicastManager manages libp2p stream negotiation and creation, which is utilized for unicast dispatches.
type UnicastManager interface {
	// WithDefaultHandler sets the default stream handler for this unicast manager. The default handler is utilized
	// as the core handler for other unicast protocols, e.g., compressions.
	SetDefaultHandler(defaultHandler libp2pnet.StreamHandler)
	// Register registers given protocol name as preferred unicast. Each invocation of register prioritizes the current protocol
	// over previously registered ones.
	// All errors returned from this function can be considered benign.
	Register(unicast protocols.ProtocolName) error
	// CreateStream tries establishing a libp2p stream to the remote peer id. It tries creating streams in the descending order of preference until
	// it either creates a successful stream or runs out of options. Creating stream on each protocol is tried at most `maxAttempts`, and then falls
	// back to the less preferred one.
	// All errors returned from this function can be considered benign.
	CreateStream(ctx context.Context, peerID peer.ID) (libp2pnet.Stream, error)
}
