package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertIndex persists the given index keyed by the block ID
//
// CAUTION:
//   - The caller must acquire the [storage.LockInsertBlock] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// No errors expected during normal operations.
func InsertIndex(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, index *flow.Index) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}

	// The following database operations are all indexing data by block ID. If used correctly,
	// we don't need to check here if the data is already stored, because the same check should have
	// been done when storing the block header, which is in the same batch update and holding the same lock.
	// If there is no header stored for the block ID, it means no index data for the same block ID
	// was stored either, as long as the same lock is held, the data is guaranteed to be consistent.
	w := rw.Writer()
	err := IndexPayloadGuarantees(lctx, w, blockID, index.GuaranteeIDs)
	if err != nil {
		return fmt.Errorf("could not store guarantee index: %w", err)
	}
	err = IndexPayloadSeals(lctx, w, blockID, index.SealIDs)
	if err != nil {
		return fmt.Errorf("could not store seal index: %w", err)
	}
	err = IndexPayloadReceipts(lctx, w, blockID, index.ReceiptIDs)
	if err != nil {
		return fmt.Errorf("could not store receipts index: %w", err)
	}
	err = IndexPayloadResults(lctx, w, blockID, index.ResultIDs)
	if err != nil {
		return fmt.Errorf("could not store results index: %w", err)
	}
	err = IndexPayloadProtocolStateID(lctx, w, blockID, index.ProtocolStateID)
	if err != nil {
		return fmt.Errorf("could not store protocol state id: %w", err)
	}
	return nil
}

func RetrieveIndex(r storage.Reader, blockID flow.Identifier, index *flow.Index) error {
	var guaranteeIDs []flow.Identifier
	err := LookupPayloadGuarantees(r, blockID, &guaranteeIDs)
	if err != nil {
		return fmt.Errorf("could not retrieve guarantee index: %w", err)
	}
	var sealIDs []flow.Identifier
	err = LookupPayloadSeals(r, blockID, &sealIDs)
	if err != nil {
		return fmt.Errorf("could not retrieve seal index: %w", err)
	}
	var receiptIDs []flow.Identifier
	err = LookupPayloadReceipts(r, blockID, &receiptIDs)
	if err != nil {
		return fmt.Errorf("could not retrieve receipts index: %w", err)
	}
	var resultsIDs []flow.Identifier
	err = LookupPayloadResults(r, blockID, &resultsIDs)
	if err != nil {
		return fmt.Errorf("could not retrieve results index: %w", err)
	}
	var stateID flow.Identifier
	err = LookupPayloadProtocolStateID(r, blockID, &stateID)
	if err != nil {
		return fmt.Errorf("could not retrieve protocol state id: %w", err)
	}

	*index = flow.Index{
		GuaranteeIDs:    guaranteeIDs,
		SealIDs:         sealIDs,
		ReceiptIDs:      receiptIDs,
		ResultIDs:       resultsIDs,
		ProtocolStateID: stateID,
	}
	return nil
}
