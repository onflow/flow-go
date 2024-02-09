package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/operation"
)

func InsertIndex(blockID flow.Identifier, index *flow.Index) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {
		err := operation.IndexPayloadGuarantees(blockID, index.CollectionIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not store guarantee index: %w", err)
		}
		err = operation.IndexPayloadSeals(blockID, index.SealIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not store seal index: %w", err)
		}
		err = operation.IndexPayloadReceipts(blockID, index.ReceiptIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not store receipts index: %w", err)
		}
		err = operation.IndexPayloadResults(blockID, index.ResultIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not store results index: %w", err)
		}
		err = operation.IndexPayloadProtocolStateID(blockID, index.ProtocolStateID)(tx)
		if err != nil {
			return fmt.Errorf("could not store protocol state id: %w", err)
		}
		return nil
	}
}

func RetrieveIndex(blockID flow.Identifier, index *flow.Index) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {
		var collIDs []flow.Identifier
		err := operation.LookupPayloadGuarantees(blockID, &collIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve guarantee index: %w", err)
		}
		var sealIDs []flow.Identifier
		err = operation.LookupPayloadSeals(blockID, &sealIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve seal index: %w", err)
		}
		var receiptIDs []flow.Identifier
		err = operation.LookupPayloadReceipts(blockID, &receiptIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve receipts index: %w", err)
		}
		var resultsIDs []flow.Identifier
		err = operation.LookupPayloadResults(blockID, &resultsIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve receipts index: %w", err)
		}
		var stateID flow.Identifier
		err = operation.LookupPayloadProtocolStateID(blockID, &stateID)(tx)
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
}
