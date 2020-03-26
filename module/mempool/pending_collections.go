package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// PendingReceipts represents a concurrency-safe memory pool for pending execution receipts.
type PendingReceipts interface {

	// Add will add the given pending receipt for the node with the given ID.
	Add(receipt PendingReceipts) error

	// Has checks if the given receipt is part of the memory pool.
	Has(receiptID flow.Identifier) bool

	// Rem will remove a receipt by ID.
	Rem(receiptID flow.Identifier) bool

	// All will return a list of all receipts in the memory pool.
	All() []*flow.ExecutionReceipt
}
