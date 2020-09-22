package verification

import (
	"context"

	"github.com/dapperlabs/flow-go/model/flow"
)

// ReceiptDataPack represents an execution receipt with some metadata.
// This is an internal entity for verification node.
type ReceiptDataPack struct {
	Receipt  *flow.ExecutionReceipt
	OriginID flow.Identifier
	Ctx      context.Context // used for span tracing
}

// ID returns the unique identifier for the ReceiptDataPack which is the
// id of its execution receipt.
func (r *ReceiptDataPack) ID() flow.Identifier {
	return r.Receipt.ID()
}

// Checksum returns the checksum of the ReceiptDataPack.
func (r *ReceiptDataPack) Checksum() flow.Identifier {
	return flow.MakeID(r)
}
