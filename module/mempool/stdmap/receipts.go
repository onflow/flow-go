package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// Receipts implements the execution receipts memory pool of the consensus node,
// used to store execution receipts and to generate block seals.
// Stored Receipts are keyed by the ID of execution receipt.
type Receipts struct {
	*Backend[flow.Identifier, *flow.ExecutionReceipt]
}

// NewReceipts creates a new memory pool for execution receipts.
func NewReceipts(limit uint) *Receipts {
	// create the receipts memory pool with the lookup maps
	return &Receipts{NewBackend(WithLimit[flow.Identifier, *flow.ExecutionReceipt](limit))}
}
