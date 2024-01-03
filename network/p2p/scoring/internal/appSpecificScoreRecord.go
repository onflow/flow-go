package internal

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// appSpecificScoreRecordEntity is an entity that represents the application specific score of a peer.
type appSpecificScoreRecordEntity struct {
	// entityId is the key of the entity in the cache. It is the hash of the peer id.
	// It is intentionally encoded as part of the struct to avoid recomputing it.
	entityId flow.Identifier

	// PeerID is the peer id of the peer that is the owner of the subscription.
	PeerID peer.ID

	// Score is the application specific score of the peer.
	Score float64

	// LastUpdated is the last time the score was updated.
	LastUpdated time.Time
}

var _ flow.Entity = (*appSpecificScoreRecordEntity)(nil)

// ID returns the entity id of the subscription record, which is the hash of the peer id.
// The AppSpecificScoreCache uses the entity id as the key in the cache.
func (a appSpecificScoreRecordEntity) ID() flow.Identifier {
	return a.entityId
}

// Checksum returns the entity id of the subscription record, which is the hash of the peer id.
// It is of no use in the cache, but it is implemented to satisfy the flow.Entity interface.
func (a appSpecificScoreRecordEntity) Checksum() flow.Identifier {
	return a.entityId
}
