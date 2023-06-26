package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerUpdater connects to the given peer.IDs. It also disconnects from any other peers with which it may have
// previously established connection.
type PeerUpdater interface {
	// UpdatePeers connects to the given peer.IDs. It also disconnects from any other peers with which it may have
	// previously established connection.
	// UpdatePeers implementation should be idempotent such that multiple calls to connect to the same peer should not
	// create multiple connections
	UpdatePeers(ctx context.Context, peerIDs peer.IDSlice)
}

// Connector is an interface that allows connecting to a peer.ID.
type Connector interface {
	// Connect connects to the given peer.ID.
	// Note that connection may be established asynchronously. Any error encountered while connecting to the peer.ID
	// is benign and should not be returned. Also, Connect implementation should not cause any blocking or crash.
	// Args:
	// 	ctx: context.Context to be used for the connection
	// 	peerChan: channel to which the peer.AddrInfo of the connected peer.ID is sent.
	// Returns:
	//  none.
	Connect(ctx context.Context, peerChan <-chan peer.AddrInfo)
}

// ConnectorFactory is a factory function to create a new Connector.
type ConnectorFactory func(host host.Host) (Connector, error)

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

	// IsConnectedTo returns true if the given peer.ID is connected to the underlying host.
	// Args:
	// 	peerID: peer.ID for which the connection status is requested
	// Returns:
	// 	true if the given peer.ID is connected to the underlying host.
	IsConnectedTo(peerId peer.ID) bool

	// PeerInfo returns the peer.AddrInfo for the given peer.ID.
	// Args:
	// 	id: peer.ID for which the peer.AddrInfo is requested
	// Returns:
	// 	peer.AddrInfo for the given peer.ID
	PeerInfo(peerId peer.ID) peer.AddrInfo

	// IsProtected returns true if the given peer.ID is protected from pruning.
	// Args:
	// 	id: peer.ID for which the protection status is requested
	// Returns:
	// 	true if the given peer.ID is protected from pruning
	IsProtected(peerId peer.ID) bool

	// ClosePeer closes the connection to the given peer.ID.
	// Args:
	// 	id: peer.ID for which the connection is to be closed
	// Returns:
	// 	error if there is any error while closing the connection to the given peer.ID. All errors are benign.
	ClosePeer(peerId peer.ID) error

	// ID returns the peer.ID of the underlying host.
	// Returns:
	// 	peer.ID of the underlying host.
	ID() peer.ID
}
