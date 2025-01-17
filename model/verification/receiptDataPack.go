package verification

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

// ReceiptDataPack represents an execution receipt with some metadata.
// This is an internal entity for verification node.
type ReceiptDataPack struct {
	Receipt  *flow.ExecutionReceipt
	OriginID flow.Identifier
	Ctx      context.Context // used for span tracing
}
