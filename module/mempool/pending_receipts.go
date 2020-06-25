package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
)

// PendingReceipts represents a concurrency-safe memory pool for pending execution receipts.
type PendingReceipts interface {

	// Add will add the given pending receipt to the memory pool. It will return
	// false if it was already in the mempool.
	Add(preceipt *verification.PendingReceipt) bool

	// Get returns the pending receipt and true, if the pending receipt is in the
	// mempool. Otherwise, it returns nil and false.
	Get(preceiptID flow.Identifier) (*verification.PendingReceipt, bool)

	// Has checks if the given receipt is part of the memory pool.
	Has(preceiptID flow.Identifier) bool

	// Rem will remove a receipt by ID.
	Rem(preceiptID flow.Identifier) bool

	// Size returns total number receipts in mempool
	Size() uint

	// All will return a list of all receipts in the memory pool.
	All() []*verification.PendingReceipt
}
