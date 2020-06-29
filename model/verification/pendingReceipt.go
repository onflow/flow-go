package verification

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// PendingReceipt represents a receipt that its origin ID is pending to be verified
// It is utilized whenever the reference blockID for the pending receipt is not available
type PendingReceipt struct {
	Receipt  *flow.ExecutionReceipt
	OriginID flow.Identifier
}

// ID returns the unique identifier for the pending receipt which is the
// id of its execution receipt.
func (p *PendingReceipt) ID() flow.Identifier {
	return p.Receipt.ID()
}

// Checksum returns the checksum of the pending receipt.
func (p *PendingReceipt) Checksum() flow.Identifier {
	return flow.MakeID(p)
}

// NewPendingReceipt creates a new PendingReceipt structure out of the receipt and
// its originID.
func NewPendingReceipt(receipt *flow.ExecutionReceipt, originID flow.Identifier) *PendingReceipt {
	return &PendingReceipt{
		Receipt:  receipt,
		OriginID: originID,
	}
}
