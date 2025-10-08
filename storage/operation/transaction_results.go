package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InsertTransactionResult(w storage.Writer, blockID flow.Identifier, transactionResult *flow.TransactionResult) error {
	return UpsertByKey(w, MakePrefix(codeTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func IndexTransactionResult(w storage.Writer, blockID flow.Identifier, txIndex uint32, transactionResult *flow.TransactionResult) error {
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

// BatchInsertAndIndexLightTransactionResults inserts and indexes a batch of light transaction results
// the caller must hold [storage.LockInsertLightTransactionResult] lock
// It returns storage.ErrAlreadyExists if light transaction results for the block already exist
func BatchInsertAndIndexLightTransactionResults(
	lctx lockctx.Proof, rw storage.ReaderBatchWriter,
	blockID flow.Identifier,
	transactionResults []flow.LightTransactionResult,
) error {
	if !lctx.HoldsLock(storage.LockInsertLightTransactionResult) {
		return fmt.Errorf("lock %s is not held", storage.LockInsertLightTransactionResult)
	}

	// ensure we don't overwrite existing light transaction results for this block
	prefix := MakePrefix(codeLightTransactionResult, blockID)
	checkExists := func(key []byte) error {
		return fmt.Errorf("light transaction results for block %s already exist: %w", blockID, storage.ErrAlreadyExists)
	}
	err := IterateKeysByPrefixRange(rw.GlobalReader(), prefix, prefix, checkExists)
	if err != nil {
		return err
	}

	w := rw.Writer()
	for i, result := range transactionResults {
		err := batchInsertLightTransactionResult(w, blockID, &result)
		if err != nil {
			return fmt.Errorf("cannot batch insert light tx result: %w", err)
		}

		err = batchIndexLightTransactionResult(w, blockID, uint32(i), &result)
		if err != nil {
			return fmt.Errorf("cannot batch index light tx result: %w", err)
		}
	}
	return nil
}

// batchInsertLightTransactionResult inserts a light transaction result by block ID and transaction ID
// into the database using a batch write.
func batchInsertLightTransactionResult(w storage.Writer, blockID flow.Identifier, transactionResult *flow.LightTransactionResult) error {
	return UpsertByKey(w, MakePrefix(codeLightTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

// batchIndexLightTransactionResult indexes a light transaction result by index within the block using a
// batch write.
func batchIndexLightTransactionResult(w storage.Writer, blockID flow.Identifier, txIndex uint32, transactionResult *flow.LightTransactionResult) error {
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

// BatchInsertAndIndexTransactionResultErrorMessages inserts and indexes a batch of transaction result error messages
// the caller must hold [storage.LockInsertTransactionResultErrMessage] lock
// It returns storage.ErrAlreadyExists if tx result error messages for the block already exist
func BatchInsertAndIndexTransactionResultErrorMessages(
	lctx lockctx.Proof, rw storage.ReaderBatchWriter,
	blockID flow.Identifier,
	transactionResultErrorMessages []flow.TransactionResultErrorMessage,
) error {
	if !lctx.HoldsLock(storage.LockInsertTransactionResultErrMessage) {
		return fmt.Errorf("lock %s is not held", storage.LockInsertTransactionResultErrMessage)
	}

	// ensure we don't overwrite existing tx result error messages for this block
	prefix := MakePrefix(codeTransactionResultErrorMessage, blockID)
	checkExists := func(key []byte) error {
		return fmt.Errorf("transaction result error messages for block %s already exist: %w", blockID, storage.ErrAlreadyExists)
	}
	err := IterateKeysByPrefixRange(rw.GlobalReader(), prefix, prefix, checkExists)
	if err != nil {
		return err
	}

	w := rw.Writer()
	for _, result := range transactionResultErrorMessages {
		err := batchInsertTransactionResultErrorMessage(w, blockID, &result)
		if err != nil {
			return fmt.Errorf("cannot batch insert tx result error message: %w", err)
		}

		err = batchIndexTransactionResultErrorMessage(w, blockID, &result)
		if err != nil {
			return fmt.Errorf("cannot batch index tx result error message: %w", err)
		}
	}
	return nil
}

// batchInsertTransactionResultErrorMessage inserts a transaction result error message by block ID and transaction ID
// into the database using a batch write.
func batchInsertTransactionResultErrorMessage(w storage.Writer, blockID flow.Identifier, transactionResultErrorMessage *flow.TransactionResultErrorMessage) error {
	return UpsertByKey(w, MakePrefix(codeTransactionResultErrorMessage, blockID, transactionResultErrorMessage.TransactionID), transactionResultErrorMessage)
}

// batchIndexTransactionResultErrorMessage indexes a transaction result error message by index within the block using a
// batch write.
func batchIndexTransactionResultErrorMessage(w storage.Writer, blockID flow.Identifier, transactionResultErrorMessage *flow.TransactionResultErrorMessage) error {
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
