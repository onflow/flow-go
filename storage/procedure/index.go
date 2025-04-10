package procedure

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

func InsertIndex(indexing *sync.Mutex, rw storage.ReaderBatchWriter, blockID flow.Identifier, index *flow.Index) error {
	indexing.Lock()
	rw.AddCallback(func(error) {
		indexing.Unlock()
	})

	w := rw.Writer()
	// TODO: Check if the blockID is already indexed
	err := operation.UnsafeIndexPayloadGuarantees(w, blockID, index.CollectionIDs)
	if err != nil {
		return fmt.Errorf("could not store guarantee index: %w", err)
	}
	err = operation.IndexPayloadSeals(w, blockID, index.SealIDs)
	if err != nil {
		return fmt.Errorf("could not store seal index: %w", err)
	}
	err = operation.IndexPayloadReceipts(w, blockID, index.ReceiptIDs)
	if err != nil {
		return fmt.Errorf("could not store receipts index: %w", err)
	}
	err = operation.IndexPayloadResults(w, blockID, index.ResultIDs)
	if err != nil {
		return fmt.Errorf("could not store results index: %w", err)
	}
	err = operation.IndexPayloadProtocolStateID(w, blockID, index.ProtocolStateID)
	if err != nil {
		return fmt.Errorf("could not store protocol state id: %w", err)
	}
	return nil
}

func RetrieveIndex(r storage.Reader, blockID flow.Identifier, index *flow.Index) error {
	var collIDs []flow.Identifier
	err := operation.LookupPayloadGuarantees(r, blockID, &collIDs)
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
		CollectionIDs:   collIDs,
		SealIDs:         sealIDs,
		ReceiptIDs:      receiptIDs,
		ResultIDs:       resultsIDs,
		ProtocolStateID: stateID,
	}
	return nil
}
