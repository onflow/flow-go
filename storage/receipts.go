// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// ExecutionReceipts holds and indexes Execution Receipts. The storage-layer
// abstraction is from the viewpoint of the network: there are multiple
// execution nodes which produce several receipts for each block. By default,
// there is no distinguished execution node (the are all equal).
type ExecutionReceipts interface {

	// Store stores an execution receipt.
	Store(receipt *flow.ExecutionReceipt) error

	// ByID retrieves an execution receipt by its ID.
	ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, error)

	// ByBlockID retrieves all known execution receipts for the given block
	// (from any Execution Node).
	ByBlockID(blockID flow.Identifier) ([]*flow.ExecutionReceipt, error)
}

// MyExecutionReceipts holds and indexes Execution Receipts.
// MyExecutionReceipts is an extension of storage.ExecutionReceipts API.
// It introduces the notion of "MY execution receipt", from the viewpoint
// of an individual Execution Node.
type MyExecutionReceipts interface {
	ExecutionReceipts

	// StoreMyReceipt stores the receipt and marks it as mine (trusted).
	StoreMyReceipt(receipt *flow.ExecutionReceipt) error

	// MyReceipt retrieves my receipt for the given block.
	MyReceipt(blockID flow.Identifier) (*flow.ExecutionReceipt, error)
}
