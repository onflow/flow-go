package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InsertTransactionResult(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, transactionResult *flow.TransactionResult) error {
	if !lctx.HoldsLock(storage.LockInsertOwnReceipt) {
		return fmt.Errorf("InsertTransactionResult requires LockInsertOwnReceipt to be held")
	}
	return UpsertByKey(w, MakePrefix(codeTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func IndexTransactionResult(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, txIndex uint32, transactionResult *flow.TransactionResult) error {
	if !lctx.HoldsLock(storage.LockInsertOwnReceipt) {
		return fmt.Errorf("IndexTransactionResult requires LockInsertOwnReceipt to be held")
	}
	return UpsertByKey(w, MakePrefix(codeTransactionResultIndex, blockID, txIndex), transactionResult)
}

func RetrieveTransactionResult(r storage.Reader, blockID flow.Identifier, transactionID flow.Identifier, transactionResult *flow.TransactionResult) error {
	return RetrieveByKey(r, MakePrefix(codeTransactionResult, blockID, transactionID), transactionResult)
}

func RetrieveTransactionResultByIndex(r storage.Reader, blockID flow.Identifier, txIndex uint32, transactionResult *flow.TransactionResult) error {
	return RetrieveByKey(r, MakePrefix(codeTransactionResultIndex, blockID, txIndex), transactionResult)
}

// LookupTransactionResultsByBlockIDUsingIndex retrieves all tx results for a block, by using
// tx_index index. This correctly handles cases of duplicate transactions within block.
func LookupTransactionResultsByBlockIDUsingIndex(r storage.Reader, blockID flow.Identifier, txResults *[]flow.TransactionResult) error {

	txErrIterFunc := func(keyCopy []byte, getValue func(destVal any) error) (bail bool, err error) {
		var val flow.TransactionResult
		err = getValue(&val)
		if err != nil {
			return true, err
		}
		*txResults = append(*txResults, val)
		return false, nil
	}

	return TraverseByPrefix(r, MakePrefix(codeTransactionResultIndex, blockID), txErrIterFunc, storage.DefaultIteratorOptions())
}

// RemoveTransactionResultsByBlockID removes the transaction results for the given blockID
func RemoveTransactionResultsByBlockID(r storage.Reader, w storage.Writer, blockID flow.Identifier) error {
	prefix := MakePrefix(codeTransactionResult, blockID)
	err := RemoveByKeyPrefix(r, w, prefix)
	if err != nil {
		return fmt.Errorf("could not remove transaction results for block %v: %w", blockID, err)
	}

	return nil
}

// BatchRemoveTransactionResultsByBlockID removes transaction results for the given blockID in a provided batch.
// No errors are expected during normal operation, but it may return generic error
// if badger fails to process request
func BatchRemoveTransactionResultsByBlockID(blockID flow.Identifier, batch storage.ReaderBatchWriter) error {
	prefix := MakePrefix(codeTransactionResult, blockID)
	err := RemoveByKeyPrefix(batch.GlobalReader(), batch.Writer(), prefix)
	if err != nil {
		return fmt.Errorf("could not remove transaction results for block %v: %w", blockID, err)
	}

	return nil
}

// deprecated
func InsertLightTransactionResult(w storage.Writer, blockID flow.Identifier, transactionResult *flow.LightTransactionResult) error {
	return UpsertByKey(w, MakePrefix(codeLightTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func BatchInsertLightTransactionResult(w storage.Writer, blockID flow.Identifier, transactionResult *flow.LightTransactionResult) error {
	return UpsertByKey(w, MakePrefix(codeLightTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func BatchIndexLightTransactionResult(w storage.Writer, blockID flow.Identifier, txIndex uint32, transactionResult *flow.LightTransactionResult) error {
	return UpsertByKey(w, MakePrefix(codeLightTransactionResultIndex, blockID, txIndex), transactionResult)
}

func RetrieveLightTransactionResult(r storage.Reader, blockID flow.Identifier, transactionID flow.Identifier, transactionResult *flow.LightTransactionResult) error {
	return RetrieveByKey(r, MakePrefix(codeLightTransactionResult, blockID, transactionID), transactionResult)
}

func RetrieveLightTransactionResultByIndex(r storage.Reader, blockID flow.Identifier, txIndex uint32, transactionResult *flow.LightTransactionResult) error {
	return RetrieveByKey(r, MakePrefix(codeLightTransactionResultIndex, blockID, txIndex), transactionResult)
}

// LookupLightTransactionResultsByBlockIDUsingIndex retrieves all tx results for a block, but using
// tx_index index. This correctly handles cases of duplicate transactions within block.
func LookupLightTransactionResultsByBlockIDUsingIndex(r storage.Reader, blockID flow.Identifier, txResults *[]flow.LightTransactionResult) error {

	txErrIterFunc := func(keyCopy []byte, getValue func(destVal any) error) (bail bool, err error) {
		var val flow.LightTransactionResult
		err = getValue(&val)
		if err != nil {
			return true, err
		}
		*txResults = append(*txResults, val)
		return false, nil
	}

	return TraverseByPrefix(r, MakePrefix(codeLightTransactionResultIndex, blockID), txErrIterFunc, storage.DefaultIteratorOptions())
}

// BatchInsertTransactionResultErrorMessage inserts a transaction result error message by block ID and transaction ID
// into the database using a batch write.
func BatchInsertTransactionResultErrorMessage(w storage.Writer, blockID flow.Identifier, transactionResultErrorMessage *flow.TransactionResultErrorMessage) error {
	return UpsertByKey(w, MakePrefix(codeTransactionResultErrorMessage, blockID, transactionResultErrorMessage.TransactionID), transactionResultErrorMessage)
}

// BatchIndexTransactionResultErrorMessage indexes a transaction result error message by index within the block using a
// batch write.
func BatchIndexTransactionResultErrorMessage(w storage.Writer, blockID flow.Identifier, transactionResultErrorMessage *flow.TransactionResultErrorMessage) error {
	return UpsertByKey(w, MakePrefix(codeTransactionResultErrorMessageIndex, blockID, transactionResultErrorMessage.Index), transactionResultErrorMessage)
}

// RetrieveTransactionResultErrorMessage retrieves a transaction result error message by block ID and transaction ID.
func RetrieveTransactionResultErrorMessage(r storage.Reader, blockID flow.Identifier, transactionID flow.Identifier, transactionResultErrorMessage *flow.TransactionResultErrorMessage) error {
	return RetrieveByKey(r, MakePrefix(codeTransactionResultErrorMessage, blockID, transactionID), transactionResultErrorMessage)
}

// RetrieveTransactionResultErrorMessageByIndex retrieves a transaction result error message by block ID and index.
func RetrieveTransactionResultErrorMessageByIndex(r storage.Reader, blockID flow.Identifier, txIndex uint32, transactionResultErrorMessage *flow.TransactionResultErrorMessage) error {
	return RetrieveByKey(r, MakePrefix(codeTransactionResultErrorMessageIndex, blockID, txIndex), transactionResultErrorMessage)
}

// TransactionResultErrorMessagesExists checks whether tx result error messages exist in the database.
func TransactionResultErrorMessagesExists(r storage.Reader, blockID flow.Identifier, blockExists *bool) error {
	exists, err := KeyExists(r, MakePrefix(codeTransactionResultErrorMessageIndex, blockID))
	if err != nil {
		return err
	}
	*blockExists = exists
	return nil
}

// LookupTransactionResultErrorMessagesByBlockIDUsingIndex retrieves all tx result error messages for a block, by using
// tx_index index. This correctly handles cases of duplicate transactions within block.
func LookupTransactionResultErrorMessagesByBlockIDUsingIndex(r storage.Reader, blockID flow.Identifier, txResultErrorMessages *[]flow.TransactionResultErrorMessage) error {
	txErrIterFunc := func(keyCopy []byte, getValue func(destVal any) error) (bail bool, err error) {
		var val flow.TransactionResultErrorMessage
		err = getValue(&val)
		if err != nil {
			return true, err
		}
		*txResultErrorMessages = append(*txResultErrorMessages, val)
		return false, nil
	}

	return TraverseByPrefix(r, MakePrefix(codeTransactionResultErrorMessageIndex, blockID), txErrIterFunc, storage.DefaultIteratorOptions())
}
