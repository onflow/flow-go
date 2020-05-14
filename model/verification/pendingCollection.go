package verification

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// TODO consolidate with PendingReceipt to preserve DRY
// https://github.com/dapperlabs/flow-go/issues/3690
// PendingCollection represents a collection that its origin ID is pending to be verified
// It is utilized whenever the reference blockID for the pending collection is not available
type PendingCollection struct {
	Collection *flow.Collection
	OriginID   flow.Identifier
}

// ID returns the unique identifier for the pending receipt which is the
// id of its collection
func (p *PendingCollection) ID() flow.Identifier {
	return p.Collection.ID()
}

// Checksum returns the checksum of the pending collection.
func (p *PendingCollection) Checksum() flow.Identifier {
	return flow.MakeID(p)
}

// NewPendingCollection creates a new PendingCollection structure out of the collection and
// its originID.
func NewPendingCollection(collection *flow.Collection, originID flow.Identifier) *PendingCollection {
	return &PendingCollection{
		Collection: collection,
		OriginID:   originID,
	}
}
