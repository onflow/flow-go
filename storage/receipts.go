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
	BatchStore(receipt *flow.ExecutionReceipt, batch ReaderBatchWriter) error

	// ByID retrieves an execution receipt by its ID.
	ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, error)

	// ByBlockID retrieves all known execution receipts for the given block
	// (from any Execution Node).
	//
	// No errors are expected errors during normal operations.
	ByBlockID(blockID flow.Identifier) (flow.ExecutionReceiptList, error)
}

// MyExecutionReceipts reuses the storage.ExecutionReceipts API, but doesn't expose
// them. Instead, it includes the "My" in the method name in order to highlight the notion
// of "MY execution receipt", from the viewpoint of an individual Execution Node.
type MyExecutionReceipts interface {
	// BatchStoreMyReceipt stores blockID-to-my-receipt index entry keyed by blockID in a provided batch.
	// No errors are expected during normal operation
	// If entity fails marshalling, the error is wrapped in a generic error and returned.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchStoreMyReceipt(receipt *flow.ExecutionReceipt, batch ReaderBatchWriter) error

	// MyReceipt retrieves my receipt for the given block.
	MyReceipt(blockID flow.Identifier) (*flow.ExecutionReceipt, error)

	// BatchRemoveIndexByBlockID removes blockID-to-my-execution-receipt index entry keyed by a blockID in a provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemoveIndexByBlockID(blockID flow.Identifier, batch ReaderBatchWriter) error
}
