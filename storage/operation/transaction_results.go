package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertAndIndexTransactionResults inserts and indexes multiple transaction results in a single batch write.
// Conceptually, for a block this data should be written once and never changed. This is enforced by the
// function, for which reason the caller must hold the [storage.LockInsertAndIndexTxResult] lock.
// It returns [storage.ErrAlreadyExists] if transaction results for the block already exist.
func InsertAndIndexTransactionResults(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, transactionResults []flow.TransactionResult) error {
	if !lctx.HoldsLock(storage.LockInsertAndIndexTxResult) {
		return fmt.Errorf("InsertTransactionResult requires LockInsertAndIndexTxResult to be held")
	}

	// Check if transaction results for the block already exist
	// We can exit early if we find one existing transaction result R, assuming that the process which wrote R in the past
	// correctly inserted all other results for the block containing R.
	// This function only inserts new transaction results; it does not sanity check existing results or ever overwrite results.

	prefix := MakePrefix(codeTransactionResult, blockID)
	checkExists := func(key []byte) error {
		return fmt.Errorf("transaction results for block %v already exist: %w", blockID, storage.ErrAlreadyExists)
	}
	err := IterateKeysByPrefixRange(rw.GlobalReader(), prefix, prefix, checkExists)
	if err != nil {
		return err
	}

	// there is no existing transaction result for the block, we can proceed to insert
	w := rw.Writer()
	for i, result := range transactionResults {
		err := insertTransactionResult(w, blockID, &result)
		if err != nil {
			return fmt.Errorf("cannot batch insert tx result: %w", err)
		}

		err = indexTransactionResult(w, blockID, uint32(i), &result)
		if err != nil {
			return fmt.Errorf("cannot batch index tx result: %w", err)
		}
	}

	return nil
}

func insertTransactionResult(w storage.Writer, blockID flow.Identifier, transactionResult *flow.TransactionResult) error {
	return UpsertByKey(w, MakePrefix(codeTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func indexTransactionResult(w storage.Writer, blockID flow.Identifier, txIndex uint32, transactionResult *flow.TransactionResult) error {
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

// RemoveTransactionResultsByBlockID removes transaction results for the given blockID in a provided batch.
// No errors are expected during normal operation, but it may return generic error
// if badger fails to process request
func RemoveTransactionResultsByBlockID(blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
	prefix := MakePrefix(codeTransactionResult, blockID)
	err := RemoveByKeyPrefix(rw.GlobalReader(), rw.Writer(), prefix)
	if err != nil {
		return fmt.Errorf("could not remove transaction results for block %v: %w", blockID, err)
	}

	return nil
}

// InsertAndIndexLightTransactionResults persists and indexes all transaction results (light representation) for the given blockID
// as part of the provided batch. The caller must acquire [storage.LockInsertLightTransactionResult] and hold it until the write
// batch has been committed.
// It returns [storage.ErrAlreadyExists] if light transaction results for the block already exist.
func InsertAndIndexLightTransactionResults(
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
		// inserts a light transaction result by block ID and transaction ID
		err := UpsertByKey(w, MakePrefix(codeLightTransactionResult, blockID, result.TransactionID), &result)
		if err != nil {
			return fmt.Errorf("cannot batch insert light tx result: %w", err)
		}
		// indexes a light transaction result by index within the block
		err = UpsertByKey(w, MakePrefix(codeLightTransactionResultIndex, blockID, uint32(i)), &result)
		if err != nil {
			return fmt.Errorf("cannot batch index light tx result: %w", err)
		}
	}
	return nil
}

// RetrieveLightTransactionResult retrieves the result (light representation) of the specified transaction
// within the specified block.
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if no result of a transaction with the specified ID in `blockID` is known
func RetrieveLightTransactionResult(r storage.Reader, blockID flow.Identifier, transactionID flow.Identifier, transactionResult *flow.LightTransactionResult) error {
	return RetrieveByKey(r, MakePrefix(codeLightTransactionResult, blockID, transactionID), transactionResult)
}

// RetrieveLightTransactionResultByIndex retrieves the result (light representation) of the
// transaction at the given index within the specified block.
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if no result of a transaction at `txIndex` in `blockID` is known
func RetrieveLightTransactionResultByIndex(r storage.Reader, blockID flow.Identifier, txIndex uint32, transactionResult *flow.LightTransactionResult) error {
	return RetrieveByKey(r, MakePrefix(codeLightTransactionResultIndex, blockID, txIndex), transactionResult)
}

// LookupLightTransactionResultsByBlockIDUsingIndex retrieves all tx results for the specified block.
// CAUTION: this function returns the empty list in case for block IDs without known results.
// No error returns are expected during normal operations.
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

// InsertAndIndexTransactionResultErrorMessages persists and indexes all transaction result error messages for the given blockID
// as part of the provided batch. The caller must acquire [storage.LockInsertTransactionResultErrMessage] and hold it until the
// write batch has been committed.
// It returns [storage.ErrAlreadyExists] if tx result error messages for the block already exist.
func InsertAndIndexTransactionResultErrorMessages(
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
	for _, txErrMsg := range transactionResultErrorMessages {
		// insertTransactionResultErrorMessageByTxID inserts a transaction result error message by block ID and transaction ID
		err := UpsertByKey(w, MakePrefix(codeTransactionResultErrorMessage, blockID, txErrMsg.TransactionID), &txErrMsg)
		if err != nil {
			return fmt.Errorf("cannot batch insert tx result error message: %w", err)
		}
		// indexTransactionResultErrorMessageBlockIDTxIndex indexes a transaction result error message by index within the block
		err = UpsertByKey(w, MakePrefix(codeTransactionResultErrorMessageIndex, blockID, txErrMsg.Index), &txErrMsg)
		if err != nil {
			return fmt.Errorf("cannot batch index tx result error message: %w", err)
		}
	}
	return nil
}

// RetrieveTransactionResultErrorMessage retrieves a transaction result error message of the specified transaction
// within the specified block.
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if no result error message of a transaction with the specified ID in `blockID` is known
func RetrieveTransactionResultErrorMessage(r storage.Reader, blockID flow.Identifier, transactionID flow.Identifier, transactionResultErrorMessage *flow.TransactionResultErrorMessage) error {
	return RetrieveByKey(r, MakePrefix(codeTransactionResultErrorMessage, blockID, transactionID), transactionResultErrorMessage)
}

// RetrieveTransactionResultErrorMessageByIndex retrieves the transaction result error message of the
// transaction at the given index within the specified block.
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if no result of a transaction at `txIndex` in `blockID` is known
func RetrieveTransactionResultErrorMessageByIndex(r storage.Reader, blockID flow.Identifier, txIndex uint32, transactionResultErrorMessage *flow.TransactionResultErrorMessage) error {
	return RetrieveByKey(r, MakePrefix(codeTransactionResultErrorMessageIndex, blockID, txIndex), transactionResultErrorMessage)
}

// TransactionResultErrorMessagesExists checks whether tx result error messages exist in the database.
// No error returns are expected during normal operations.
func TransactionResultErrorMessagesExists(r storage.Reader, blockID flow.Identifier, blockExists *bool) error {
	exists, err := KeyExists(r, MakePrefix(codeTransactionResultErrorMessageIndex, blockID))
	if err != nil {
		return err
	}
	*blockExists = exists
	return nil
}

// LookupTransactionResultErrorMessagesByBlockIDUsingIndex retrieves the transaction result error messages of all
// failed transactions for the specified block.
// CAUTION: This method returns an empty slice if transaction results/errors for the block are not indexed yet OR if the block does not have any errors.
// No error returns are expected during normal operations.
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
