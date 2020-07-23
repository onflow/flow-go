package messages

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type EntityRequest struct {
	Nonce     uint64
	EntityIDs []flow.Identifier
}

type EntityResponse struct {
	Nonce uint64
	Blobs [][]byte
}

// ExecutionReceiptByBlockID represents a receipt entity that is requested
// by block ID.
type ExecutionReceiptByBlockID struct {
	Receipt *flow.ExecutionReceipt
}

// ID returns the block ID of the execution receipt, which is the identifier
// used to request.
func (er *ExecutionReceiptByBlockID) ID() flow.Identifier {
	return er.Receipt.ExecutionResult.BlockID
}

func (er *ExecutionReceiptByBlockID) Checksum() flow.Identifier {
	return er.Receipt.ID()
}
