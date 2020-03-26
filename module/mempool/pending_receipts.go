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

	// ByBlockID will return all execution receipts for the given block.
	ByBlockID(blockID flow.Identifier) []*flow.ExecutionReceipt

	// DropForBlock will drop all receipts for the given block ID.
	DropForBlock(blockID flow.Identifier)

	// All will return a list of all receipts in the memory pool.
	All() []*flow.ExecutionReceipt
}
