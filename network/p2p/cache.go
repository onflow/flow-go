package p2p

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// ProtocolPeerCache is an interface that stores a mapping from protocol ID to peers who support that protocol.
type ProtocolPeerCache interface {
	// RemovePeer removes the specified peer from the protocol cache.
	RemovePeer(peerID peer.ID)

	// AddProtocols adds the specified protocols for the given peer to the protocol cache.
	AddProtocols(peerID peer.ID, protocols []protocol.ID)

	// RemoveProtocols removes the specified protocols for the given peer from the protocol cache.
	RemoveProtocols(peerID peer.ID, protocols []protocol.ID)

	// GetPeers returns a copy of the set of peers that support the given protocol.
	GetPeers(pid protocol.ID) map[peer.ID]struct{}
}
