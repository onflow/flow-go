package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

// ExecutionReceipts holds and indexes Execution Receipts. The storage-layer
// abstraction is from the viewpoint of the network: there are multiple
// execution nodes which produce several receipts for each block. By default,
// there is no distinguished execution node (they are all equal).
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

	// BatchStoreMyReceipt stores the receipt by its ID and populates the index blockID → receiptID in the provided batch.
	//
	// CAUTION: By persisting the receipt, the Execution Node is effectively committing to this being the correct result.
	// Changing data could cause the node to publish inconsistent commitments and to be slashed, or the protocol to be
	// compromised as a whole. Therefore, the function checks upfront that we are not changing a previously stored
	// commitment. This function is idempotent, i.e. repeated calls with the *initially* indexed result are no-ops.
	// To guarantee atomicity of existence-check plus database write, we require the caller to acquire
	// the [storage.LockInsertMyReceipt] lock and hold it until the database write has been committed.
	//
	// Expected error returns during *normal* operations:
	//   - [storage.ErrDataMismatch] if a *different* receipt has already been indexed for the same block
	BatchStoreMyReceipt(lctx lockctx.Proof, receipt *flow.ExecutionReceipt, batch ReaderBatchWriter) error

	// MyReceipt retrieves my receipt for the given block.
	MyReceipt(blockID flow.Identifier) (*flow.ExecutionReceipt, error)

	// BatchRemoveIndexByBlockID removes blockID-to-my-execution-receipt index entry keyed by a blockID in a provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If database unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemoveIndexByBlockID(blockID flow.Identifier, batch ReaderBatchWriter) error
}
