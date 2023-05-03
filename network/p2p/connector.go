package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p/core/network"
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

type PeerFilter func(peer.ID) error

// AllowAllPeerFilter returns a peer filter that does not do any filtering.
func AllowAllPeerFilter() PeerFilter {
	return func(p peer.ID) error {
		return nil
	}
}

// ConnectorHost is a wrapper around the libp2p host.Host interface to provide the required functionality for the
// Connector interface.
type ConnectorHost interface {
	// Connections returns all the connections of the underlying host.
	Connections() []network.Conn

	// PeerInfo returns the peer.AddrInfo for the given peer.ID.
	// Args:
	// 	id: peer.ID for which the peer.AddrInfo is requested
	// Returns:
	// 	peer.AddrInfo for the given peer.ID
	PeerInfo(id peer.ID) peer.AddrInfo

	// IsProtected returns true if the given peer.ID is protected from pruning.
	// Args:
	// 	id: peer.ID for which the protection status is requested
	// Returns:
	// 	true if the given peer.ID is protected from pruning
	IsProtected(id peer.ID) bool

	// ClosePeer closes the connection to the given peer.ID.
	// Args:
	// 	id: peer.ID for which the connection is to be closed
	// Returns:
	// 	error if there is any error while closing the connection to the given peer.ID. All errors are benign.
	ClosePeer(id peer.ID) error

	// ID returns the peer.ID of the underlying host.
	// Returns:
	// 	peer.ID of the underlying host.
	ID() peer.ID
}
