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

	// BatchStore stores an execution receipt inside given batch
	BatchStore(receipt *flow.ExecutionReceipt, batch BatchStorage) error

	// RemoveByBlockID receipt by block ID
	RemoveByBlockID(blockID flow.Identifier) error

	// ByID retrieves an execution receipt by its ID.
	ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, error)

	// ByBlockID retrieves all known execution receipts for the given block
	// (from any Execution Node).
	ByBlockID(blockID flow.Identifier) (flow.ExecutionReceiptList, error)
}

// MyExecutionReceipts reuses the storage.ExecutionReceipts API, but doesn't expose
// them. Instead, it includes the "My" in the method name in order to highlight the notion
// of "MY execution receipt", from the viewpoint of an individual Execution Node.
type MyExecutionReceipts interface {
	// StoreMyReceipt stores the receipt and marks it as mine (trusted). My
	// receipts are indexed by the block whose result they compute. Currently,
	// we only support indexing a _single_ receipt per block. Attempting to
	// store conflicting receipts for the same block will error.
	StoreMyReceipt(receipt *flow.ExecutionReceipt) error

	// BatchStoreMyReceipt stores the receipt and marks it as mine (trusted) in a given batch
	BatchStoreMyReceipt(receipt *flow.ExecutionReceipt, batch BatchStorage) error

	// MyReceipt retrieves my receipt for the given block.
	MyReceipt(blockID flow.Identifier) (*flow.ExecutionReceipt, error)
}
