package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertExecutionReceiptStub inserts a [flow.ExecutionReceiptStub] into the database, keyed by its ID.
//
// CAUTION: The caller must ensure receiptID is a collision-resistant hash of the provided
// [flow.ExecutionReceiptMeta]! This method silently overrides existing data, which is safe only if
// for the same key, we always write the same value.
func InsertExecutionReceiptStub(w storage.Writer, receiptID flow.Identifier, meta *flow.ExecutionReceiptStub) error {
	return UpsertByKey(w, MakePrefix(codeExecutionReceiptMeta, receiptID), meta)
}

// RetrieveExecutionReceiptStub retrieves a [flow.ExecutionReceiptStub] by its ID.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no receipt stub with the specified ID is known.
func RetrieveExecutionReceiptStub(r storage.Reader, receiptID flow.Identifier, meta *flow.ExecutionReceiptStub) error {
	return RetrieveByKey(r, MakePrefix(codeExecutionReceiptMeta, receiptID), meta)
}

// IndexOwnExecutionReceipt indexes the Execution Node's OWN execution receipt by the executed block ID.
//
// Error returns:
//   - [storage.ErrDataMismatch] if a *different* receipt has already been indexed for the same block
func IndexOwnExecutionReceipt(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, receiptID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertMyReceipt) {
		return fmt.Errorf("cannot index own execution receipt without holding lock %s", storage.LockInsertMyReceipt)
	}

	key := MakePrefix(codeOwnBlockReceipt, blockID)

	var existing flow.Identifier
	err := RetrieveByKey(rw.GlobalReader(), key, &existing)
	if err == nil {
		if existing != receiptID {
			return fmt.Errorf("own execution receipt for block %v already exists with different value, (existing: %v, new: %v), %w", blockID, existing, receiptID, storage.ErrDataMismatch)
		}
		return nil // The receipt already exists, no need to index again
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not check existing own execution receipt: %w", err)
	}

	return UpsertByKey(rw.Writer(), key, receiptID)
}

// LookupOwnExecutionReceipt retrieves the Execution Node's OWN execution receipt ID for the specified block.
// Intended for Execution Node only. For every block executed by this node, this index should be populated.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `blockID` does not refer to a block executed by this node
func LookupOwnExecutionReceipt(r storage.Reader, blockID flow.Identifier, receiptID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeOwnBlockReceipt, blockID), receiptID)
}

// RemoveOwnExecutionReceipt removes the Execution Node's OWN execution receipt index for the given block ID.
// CAUTION: this is for recovery purposes only, and should not be used during normal operations!
// It returns nil if the collection does not exist.
//
// No errors are expected during normal operation.
func RemoveOwnExecutionReceipt(w storage.Writer, blockID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeOwnBlockReceipt, blockID))
}

// IndexExecutionReceipts adds the given execution receipts to the set of all known receipts for the
// given block. It produces a mapping from block ID to the set of all known receipts for that block.
// One block could have multiple receipts, even if they are from the same executor.
//
// This method is idempotent, and can be called repeatedly with the same block ID and receipt ID,
// without the risk of data corruption.
//
// No errors are expected during normal operation.
func IndexExecutionReceipts(w storage.Writer, blockID, receiptID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeAllBlockReceipts, blockID, receiptID), receiptID)
}

// LookupExecutionReceipts retrieves the set of all execution receipts for the specified block.
// For every known block (at or above the root block height), this index should be populated
// with all known receipts for that block.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `blockID` does not refer to a known block
func LookupExecutionReceipts(r storage.Reader, blockID flow.Identifier, receiptIDs *[]flow.Identifier) error {
	iterationFunc := receiptIterationFunc(receiptIDs)
	return TraverseByPrefix(r, MakePrefix(codeAllBlockReceipts, blockID), iterationFunc, storage.DefaultIteratorOptions())
}

// receiptIterationFunc returns an iteration function which collects all receipt IDs found during traversal.
func receiptIterationFunc(receiptIDs *[]flow.Identifier) IterationFunc {
	return func(keyCopy []byte, getValue func(destVal any) error) (bail bool, err error) {
		var receiptID flow.Identifier
		err = getValue(&receiptID)
		if err != nil {
			return true, err
		}
		*receiptIDs = append(*receiptIDs, receiptID)
		return false, nil
	}
}
