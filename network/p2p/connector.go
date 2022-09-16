package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Connector connects to peer and disconnects from peer using the underlying networking library
type Connector interface {
	// UpdatePeers connects to the given peer.IDs. It also disconnects from any other peers with which it may have
	// previously established connection.
	// UpdatePeers implementation should be idempotent such that multiple calls to connect to the same peer should not
	// create multiple connections
	UpdatePeers(ctx context.Context, peerIDs peer.IDSlice)
}
