package connection

import (
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p"
)

// ConnectorHost is a wrapper around the libp2p host.Host interface to provide the required functionality for the
// Connector interface.
type ConnectorHost struct {
	h host.Host
}

var _ p2p.ConnectorHost = (*ConnectorHost)(nil)

func NewConnectorHost(h host.Host) *ConnectorHost {
	return &ConnectorHost{
		h: h,
	}
}

// Connections returns all the connections of the underlying host.
func (c *ConnectorHost) Connections() []network.Conn {
	return c.h.Network().Conns()
}

// IsConnectedTo returns true if the given peer.ID is connected to the underlying host.
// Args:
//
//	peerID: peer.ID for which the connection status is requested
//
// Returns:
//
//	true if the given peer.ID is connected to the underlying host.
func (c *ConnectorHost) IsConnectedTo(peerID peer.ID) bool {
	return c.h.Network().Connectedness(peerID) == network.Connected && len(c.h.Network().ConnsToPeer(peerID)) > 0
}

// PeerInfo returns the peer.AddrInfo for the given peer.ID.
// Args:
//
//	id: peer.ID for which the peer.AddrInfo is requested
//
// Returns:
//
//	peer.AddrInfo for the given peer.ID
func (c *ConnectorHost) PeerInfo(id peer.ID) peer.AddrInfo {
	return c.h.Peerstore().PeerInfo(id)
}

// IsProtected returns true if the given peer.ID is protected from pruning.
// Args:
//
//	id: peer.ID for which the protection status is requested
//
// Returns:
//
//	true if the given peer.ID is protected from pruning
func (c *ConnectorHost) IsProtected(id peer.ID) bool {
	return c.h.ConnManager().IsProtected(id, "")
}

// ClosePeer closes the connection to the given peer.ID.
// Args:
//
//	id: peer.ID for which the connection is to be closed
//
// Returns:
//
//	error if there is any error while closing the connection to the given peer.ID. All errors are benign.
func (c *ConnectorHost) ClosePeer(id peer.ID) error {
	return c.h.Network().ClosePeer(id)
}

// ID returns the peer.ID of the underlying host.
// Returns:
//
//	peer.ID of the underlying host.
//
// ID returns the peer.ID of the underlying host.
func (c *ConnectorHost) ID() peer.ID {
	return c.h.ID()
}
