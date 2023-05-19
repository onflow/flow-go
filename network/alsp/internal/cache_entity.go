package internal

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/alsp/model"
)

// ProtocolSpamRecordEntity is an entity that represents a spam record. It is internally
// used by the SpamRecordCache to store the spam records in the cache.
// The identifier of this entity is the origin id of the spam record. This entails that the spam records
// are deduplicated by origin id.
type ProtocolSpamRecordEntity struct {
	model.ProtocolSpamRecord
}

var _ flow.Entity = (*ProtocolSpamRecordEntity)(nil)

// ID returns the origin id of the spam record, which is used as the unique identifier of the entity for maintenance and
// deduplication purposes in the cache.
func (p ProtocolSpamRecordEntity) ID() flow.Identifier {
	return p.OriginId
}

// Checksum returns the origin id of the spam record, it does not have any purpose in the cache.
// It is implemented to satisfy the flow.Entity interface.
func (p ProtocolSpamRecordEntity) Checksum() flow.Identifier {
	return p.OriginId
}
