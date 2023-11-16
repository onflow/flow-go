package internal

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// SubscriptionRecordEntity is an entity that represents a the list of topics a peer is subscribed to.
// It is internally used by the SubscriptionRecordCache to store the subscription records in the cache.
type SubscriptionRecordEntity struct {
	// entityId is the key of the entity in the cache. It is the hash of the peer id.
	// It is intentionally encoded as part of the struct to avoid recomputing it.
	entityId flow.Identifier

	// PeerID is the peer id of the peer that is the owner of the subscription.
	PeerID peer.ID

	// Topics is the list of topics the peer is subscribed to.
	Topics []string

	// LastUpdatedCycle is the last cycle counter value that this record was updated.
	// This is used to clean up old records' topics upon update.
	LastUpdatedCycle uint64
}

var _ flow.Entity = (*SubscriptionRecordEntity)(nil)

// ID returns the entity id of the subscription record, which is the hash of the peer id.
func (s SubscriptionRecordEntity) ID() flow.Identifier {
	return s.entityId
}

// Checksum returns the entity id of the subscription record, which is the hash of the peer id.
// It is of no use in the cache, but it is implemented to satisfy the flow.Entity interface.
func (s SubscriptionRecordEntity) Checksum() flow.Identifier {
	return s.ID()
}
