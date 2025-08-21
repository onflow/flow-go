package procedure

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

func InsertIndex(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, index *flow.Index) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}

	// The following database operations are all indexing data by block ID,
	// they don't need to check if the data is already stored, because the same check has been done
	// when storing the block header, which is in the same batch update and holding the same lock.
	// if there is no header stored for the block ID, it means no index data for the same block ID
	// was stored either, as long as the same lock is held, the data is guaranteed to be consistent.
	w := rw.Writer()
	err := operation.IndexPayloadGuarantees(lctx, w, blockID, index.GuaranteeIDs)
	if err != nil {
		return fmt.Errorf("could not store guarantee index: %w", err)
	}
	err = operation.IndexPayloadSeals(lctx, w, blockID, index.SealIDs)
	if err != nil {
		return fmt.Errorf("could not store seal index: %w", err)
	}
	err = operation.IndexPayloadReceipts(lctx, w, blockID, index.ReceiptIDs)
	if err != nil {
		return fmt.Errorf("could not store receipts index: %w", err)
	}
	err = operation.IndexPayloadResults(lctx, w, blockID, index.ResultIDs)
	if err != nil {
		return fmt.Errorf("could not store results index: %w", err)
	}
	err = operation.IndexPayloadProtocolStateID(lctx, w, blockID, index.ProtocolStateID)
	if err != nil {
		return fmt.Errorf("could not store protocol state id: %w", err)
	}
	return nil
}

func RetrieveIndex(r storage.Reader, blockID flow.Identifier, index *flow.Index) error {
	var guaranteeIDs []flow.Identifier
	err := operation.LookupPayloadGuarantees(r, blockID, &guaranteeIDs)
	if err != nil {
		return fmt.Errorf("could not retrieve guarantee index: %w", err)
	}
	var sealIDs []flow.Identifier
	err = operation.LookupPayloadSeals(r, blockID, &sealIDs)
	if err != nil {
		return fmt.Errorf("could not retrieve seal index: %w", err)
	}
	var receiptIDs []flow.Identifier
	err = operation.LookupPayloadReceipts(r, blockID, &receiptIDs)
	if err != nil {
		return fmt.Errorf("could not retrieve receipts index: %w", err)
	}
	var resultsIDs []flow.Identifier
	err = operation.LookupPayloadResults(r, blockID, &resultsIDs)
	if err != nil {
		return fmt.Errorf("could not retrieve results index: %w", err)
	}
	var stateID flow.Identifier
	err = operation.LookupPayloadProtocolStateID(r, blockID, &stateID)
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
