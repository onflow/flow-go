package internal

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// disallowListCacheEntity is the model data type for the disallow list cache.
// It represents a single peer that is disallow-listed and the reasons for it.
// The key for storing this entity is the id field which is the hash of the peerID.
// This means that the entities are deduplicated by their peerID.
type disallowListCacheEntity struct {
	peerID peer.ID
	causes map[network.DisallowListedCause]struct{}
	// id is the hash of the peerID which is used as the key for storing the entity in the cache.
	// we cache it internally to avoid hashing the peerID multiple times.
	entityId flow.Identifier
}

var _ flow.Entity = (*disallowListCacheEntity)(nil)

// ID returns the hash of the peerID which is used as the key for storing the entity in the cache.
// Returns:
// - the hash of the peerID as a flow.Identifier.
func (d *disallowListCacheEntity) ID() flow.Identifier {
	return d.entityId
}

// Checksum returns the hash of the peerID, there is no use for this method in the cache. It is implemented to satisfy
// the flow.Entity interface.
// Returns:
// - the hash of the peerID as a flow.Identifier.
func (d *disallowListCacheEntity) Checksum() flow.Identifier {
	return d.entityId
}

// makeId is a helper function for creating the id field of the disallowListCacheEntity by hashing the peerID.
// Returns:
// - the hash of the peerID as a flow.Identifier.
func makeId(peerID peer.ID) flow.Identifier {
	return flow.MakeID([]byte(peerID))
}
