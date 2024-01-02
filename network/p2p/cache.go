package p2p

import (
	"time"

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
	// Get returns the GossipSubSpamRecord of a peer from the cache.
	// Args:
	// - peerID: the peer ID of the peer in the GossipSub protocol.
	// Returns:
	// - *GossipSubSpamRecord: the GossipSubSpamRecord of the peer.
	// - error on failure to retrieve the record. The returned error is irrecoverable and indicates an exception.
	// - bool: true if the record was retrieved successfully, false otherwise.
	Get(peerID peer.ID) (*GossipSubSpamRecord, error, bool)

	// Adjust updates the GossipSub spam penalty of a peer in the cache. If the peer does not have a record in the cache, a new record is created.
	// The order of the pre-processing functions is the same as the order in which they were added to the cache.
	// Args:
	// - peerID: the peer ID of the peer in the GossipSub protocol.
	// - updateFn: the update function to be applied to the record.
	// Returns:
	// - *GossipSubSpamRecord: the updated record.
	// - error on failure to update the record. The returned error is irrecoverable and indicates an exception.
	// Note that if any of the pre-processing functions returns an error, the record is reverted to its original state (prior to applying the update function).
	Adjust(peerID peer.ID, updateFunc UpdateFunction) (*GossipSubSpamRecord, error)

	// Has returns true if the cache contains the GossipSubSpamRecord of the given peer.
	// Args:
	// - peerID: the peer ID of the peer in the GossipSub protocol.
	// Returns:
	// - bool: true if the cache contains the GossipSubSpamRecord of the given peer, false otherwise.
	Has(peerID peer.ID) bool
}

// GossipSubApplicationSpecificScoreCache is a cache for storing the application specific score of peers.
// The application specific score of a peer is used to calculate the GossipSub score of the peer; it contains the spam penalty of the peer, staking score, and subscription penalty.
// Note that none of the application specific scores, spam penalties, staking scores, and subscription penalties are shared publicly with other peers.
// Rather they are solely used by the current peer to select the peers to which it will connect on a topic mesh.
// The cache is expected to have an eject policy to remove the least recently used record when the cache is full.
// Implementation must be thread-safe, but can be blocking.
type GossipSubApplicationSpecificScoreCache interface {
	// Get returns the application specific score of a peer from the cache.
	// Args:
	// - peerID: the peer ID of the peer in the GossipSub protocol.
	// Returns:
	// - float64: the application specific score of the peer.
	// - time.Time: the time at which the score was last updated.
	// - bool: true if the score was retrieved successfully, false otherwise.
	Get(peerID peer.ID) (float64, time.Time, bool)

	// Add adds the application specific score of a peer to the cache.
	// If the peer already has a score in the cache, the score is updated.
	// Args:
	// - peerID: the peer ID of the peer in the GossipSub protocol.
	// - score: the application specific score of the peer.
	// - time: the time at which the score was last updated.
	// Returns:
	// - error on failure to add the score. The returned error is irrecoverable and indicates an exception.
	Add(peerID peer.ID, score float64, time time.Time) error
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
	// LastDecayAdjustment records the time of the most recent adjustment in the decay process for a spam record.
	// At each interval, the system evaluates and potentially adjusts the decay rate, which affects how quickly a node's penalty diminishes.
	// The decay process is multiplicative (newPenalty = decayRate * oldPenalty) and operates within a range of 0 to 1. At certain regular intervals, the decay adjustment is evaluated and if the node's penalty falls below the set threshold, the decay rate is modified by the reduction factor, such as 0.01. This modification incrementally increases the decay rate. For example, if the decay rate is `x`, adding the reduction factor results in a decay rate of `x + 0.01`, leading to a slower reduction in penalty. Thus, a higher decay rate actually slows down the recovery process, contrary to accelerating it.
	// The LastDecayAdjustment timestamp is crucial in ensuring balanced and fair penalization, especially important during periods of high message traffic to prevent unintended rapid decay of penalties for malicious nodes.
	LastDecayAdjustment time.Time
}
