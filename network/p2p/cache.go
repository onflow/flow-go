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

// UpdateFunction is a function that adjusts the GossipSub spam record of a peer.
// Args:
// - record: the GossipSubSpamRecord of the peer.
// Returns:
// - *GossipSubSpamRecord: the adjusted GossipSubSpamRecord of the peer.
type UpdateFunction func(record GossipSubSpamRecord) GossipSubSpamRecord

// GossipSubSpamRecordCache is a cache for storing the GossipSub spam records of peers.
// The spam records of peers is used to calculate the application specific score, which is part of the GossipSub score of a peer.
// Note that none of the spam records, application specific score, and GossipSub score are shared publicly with other peers.
// Rather they are solely used by the current peer to select the peers to which it will connect on a topic mesh.
//
// Implementation must be thread-safe.
type GossipSubSpamRecordCache interface {
	// Add adds the GossipSubSpamRecord of a peer to the cache.
	// Args:
	// - peerID: the peer ID of the peer in the GossipSub protocol.
	// - record: the GossipSubSpamRecord of the peer.
	//
	// Returns:
	// - bool: true if the record was added successfully, false otherwise.
	Add(peerId peer.ID, record GossipSubSpamRecord) bool

	// Get returns the GossipSubSpamRecord of a peer from the cache.
	// Args:
	// - peerID: the peer ID of the peer in the GossipSub protocol.
	// Returns:
	// - *GossipSubSpamRecord: the GossipSubSpamRecord of the peer.
	// - error on failure to retrieve the record. The returned error is irrecoverable and indicates an exception.
	// - bool: true if the record was retrieved successfully, false otherwise.
	Get(peerID peer.ID) (*GossipSubSpamRecord, error, bool)

	// Update updates the GossipSub spam penalty of a peer in the cache using the given adjust function.
	// Args:
	// - peerID: the peer ID of the peer in the GossipSub protocol.
	// - adjustFn: the adjust function to be applied to the record.
	// Returns:
	// - *GossipSubSpamRecord: the updated record.
	// - error on failure to update the record. The returned error is irrecoverable and indicates an exception.
	Update(peerID peer.ID, updateFunc UpdateFunction) (*GossipSubSpamRecord, error)

	// Has returns true if the cache contains the GossipSubSpamRecord of the given peer.
	// Args:
	// - peerID: the peer ID of the peer in the GossipSub protocol.
	// Returns:
	// - bool: true if the cache contains the GossipSubSpamRecord of the given peer, false otherwise.
	Has(peerID peer.ID) bool
}

// GossipSubSpamRecord represents spam record of a peer in the GossipSub protocol.
// It acts as a penalty card for a peer in the GossipSub protocol that keeps the
// spam penalty of the peer as well as its decay factor.
// GossipSubSpam record is used to calculate the application specific score of a peer in the GossipSub protocol.
type GossipSubSpamRecord struct {
	// Decay factor of gossipsub spam penalty.
	// The Penalty is multiplied by the Decay factor every time the Penalty is updated.
	// This is to prevent the Penalty from being stuck at a negative value.
	// Each peer has its own Decay factor based on its behavior.
	// Valid decay value is in the range [0, 1].
	Decay float64
	// Penalty is the application specific Penalty of the peer.
	Penalty float64
}

// GossipSubDuplicateMessageTrackerCache is a cache used to store the current count of duplicate messages detected
// from a peer. This count is utilized to calculate a penalty for duplicate messages, which is then applied
// to the peer's application-specific score. The duplicate message tracker decays over time to prevent perpetual
// penalization of a peer.
type GossipSubDuplicateMessageTrackerCache interface {
	// Inc increments the number of duplicate messages detected for the peer. This func is used in conjunction with the GossipSubMeshTracer and is invoked
	// each time the DuplicateMessage callback is invoked.
	// Args:
	// - peerID: the peer ID of the peer in the GossipSub protocol.
	//
	// Returns:
	// - float64: updated value for the duplicate message tracker.
	// - error: if any error was encountered during record adjustments in cache.
	Inc(peerId peer.ID) (float64, error)
	// Get returns the current number of duplicate messages encountered from a peer. The counter is decayed before being
	// returned.
	// Args:
	// - peerID: the peer ID of the peer in the GossipSub protocol.
	//
	// Returns:
	// - float64: updated value for the duplicate message tracker.
	Get(peerId peer.ID) (float64, bool, error)
}
