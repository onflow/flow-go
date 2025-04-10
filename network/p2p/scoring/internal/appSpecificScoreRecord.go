package internal

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// appSpecificScoreRecord represents the application specific score of a peer.
type appSpecificScoreRecord struct {
	// PeerID is the peer ID of the peer in the GossipSub protocol.
	PeerID peer.ID

	// Score is the application specific score of the peer.
	Score float64

	// LastUpdated is the last time the score was updated.
	LastUpdated time.Time
}
