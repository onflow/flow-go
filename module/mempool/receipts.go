// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Receipts represents a concurrency-safe memory pool for execution receipts.
type Receipts interface {

	// Add will add the given receipt for the node with the given ID.
	Add(receipt *flow.ExecutionReceipt) error

	// Has checks if the given receipt is part of the memory pool.
	Has(receiptID flow.Identifier) bool

	// Rem will remove a receipt by ID.
	Rem(receiptID flow.Identifier) bool

	// ByBlockID will return all execution receipts for the given block.
	ByBlockID(blockID flow.Identifier) []*flow.ExecutionReceipt

	// DropForBlock will drop all receipts for the given block ID.
	DropForBlock(blockID flow.Identifier)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will return a list of all receipts in the memory pool.
	All() []*flow.ExecutionReceipt
}
