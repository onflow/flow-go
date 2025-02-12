package operation

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

func InsertTransactionResult(blockID flow.Identifier, transactionResult *flow.TransactionResult) func(*badger.Txn) error {
	return insert(makePrefix(codeTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func BatchInsertTransactionResult(blockID flow.Identifier, transactionResult *flow.TransactionResult) func(batch *badger.WriteBatch) error {
	return batchWrite(makePrefix(codeTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func BatchIndexTransactionResult(blockID flow.Identifier, txIndex uint32, transactionResult *flow.TransactionResult) func(batch *badger.WriteBatch) error {
	return batchWrite(makePrefix(codeTransactionResultIndex, blockID, txIndex), transactionResult)
}

func RetrieveTransactionResult(blockID flow.Identifier, transactionID flow.Identifier, transactionResult *flow.TransactionResult) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransactionResult, blockID, transactionID), transactionResult)
}
func RetrieveTransactionResultByIndex(blockID flow.Identifier, txIndex uint32, transactionResult *flow.TransactionResult) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransactionResultIndex, blockID, txIndex), transactionResult)
}

// LookupTransactionResultsByBlockIDUsingIndex retrieves all tx results for a block, by using
// tx_index index. This correctly handles cases of duplicate transactions within block.
func LookupTransactionResultsByBlockIDUsingIndex(blockID flow.Identifier, txResults *[]flow.TransactionResult) func(*badger.Txn) error {

	txErrIterFunc := func() (checkFunc, createFunc, handleFunc) {
		check := func(_ []byte) bool {
			return true
		}
		var val flow.TransactionResult
		create := func() interface{} {
			return &val
		}
		handle := func() error {
			*txResults = append(*txResults, val)
			return nil
		}
		return check, create, handle
	}

	return traverse(makePrefix(codeTransactionResultIndex, blockID), txErrIterFunc)
}

// RemoveTransactionResultsByBlockID removes the transaction results for the given blockID
func RemoveTransactionResultsByBlockID(blockID flow.Identifier) func(*badger.Txn) error {
	return func(txn *badger.Txn) error {

		prefix := makePrefix(codeTransactionResult, blockID)
		err := removeByPrefix(prefix)(txn)
		if err != nil {
			return fmt.Errorf("could not remove transaction results for block %v: %w", blockID, err)
		}

		return nil
	}
}

// BatchRemoveTransactionResultsByBlockID removes transaction results for the given blockID in a provided batch.
// No errors are expected during normal operation, but it may return generic error
// if badger fails to process request
func BatchRemoveTransactionResultsByBlockID(blockID flow.Identifier, batch *badger.WriteBatch) func(*badger.Txn) error {
	return func(txn *badger.Txn) error {

		prefix := makePrefix(codeTransactionResult, blockID)
		err := batchRemoveByPrefix(prefix)(txn, batch)
		if err != nil {
			return fmt.Errorf("could not remove transaction results for block %v: %w", blockID, err)
		}

		return nil
	}
}

func InsertLightTransactionResult(blockID flow.Identifier, transactionResult *flow.LightTransactionResult) func(*badger.Txn) error {
	return insert(makePrefix(codeLightTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func BatchInsertLightTransactionResult(blockID flow.Identifier, transactionResult *flow.LightTransactionResult) func(batch *badger.WriteBatch) error {
	return batchWrite(makePrefix(codeLightTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func BatchIndexLightTransactionResult(blockID flow.Identifier, txIndex uint32, transactionResult *flow.LightTransactionResult) func(batch *badger.WriteBatch) error {
	return batchWrite(makePrefix(codeLightTransactionResultIndex, blockID, txIndex), transactionResult)
}

func RetrieveLightTransactionResult(blockID flow.Identifier, transactionID flow.Identifier, transactionResult *flow.LightTransactionResult) func(*badger.Txn) error {
	return retrieve(makePrefix(codeLightTransactionResult, blockID, transactionID), transactionResult)
}

func RetrieveLightTransactionResultByIndex(blockID flow.Identifier, txIndex uint32, transactionResult *flow.LightTransactionResult) func(*badger.Txn) error {
	return retrieve(makePrefix(codeLightTransactionResultIndex, blockID, txIndex), transactionResult)
}

// LookupLightTransactionResultsByBlockIDUsingIndex retrieves all tx results for a block, but using
// tx_index index. This correctly handles cases of duplicate transactions within block.
func LookupLightTransactionResultsByBlockIDUsingIndex(blockID flow.Identifier, txResults *[]flow.LightTransactionResult) func(*badger.Txn) error {

	txErrIterFunc := func() (checkFunc, createFunc, handleFunc) {
		check := func(_ []byte) bool {
			return true
		}
		var val flow.LightTransactionResult
		create := func() interface{} {
			return &val
		}
		handle := func() error {
			*txResults = append(*txResults, val)
			return nil
		}
		return check, create, handle
	}

	return traverse(makePrefix(codeLightTransactionResultIndex, blockID), txErrIterFunc)
}

// BatchInsertTransactionResultErrorMessage inserts a transaction result error message by block ID and transaction ID
// into the database using a batch write.
func BatchInsertTransactionResultErrorMessage(blockID flow.Identifier, transactionResultErrorMessage *flow.TransactionResultErrorMessage) func(batch *badger.WriteBatch) error {
	return batchWrite(makePrefix(codeTransactionResultErrorMessage, blockID, transactionResultErrorMessage.TransactionID), transactionResultErrorMessage)
}

// BatchIndexTransactionResultErrorMessage indexes a transaction result error message by index within the block using a
// batch write.
func BatchIndexTransactionResultErrorMessage(blockID flow.Identifier, transactionResultErrorMessage *flow.TransactionResultErrorMessage) func(batch *badger.WriteBatch) error {
	return batchWrite(makePrefix(codeTransactionResultErrorMessageIndex, blockID, transactionResultErrorMessage.Index), transactionResultErrorMessage)
}

// RetrieveTransactionResultErrorMessage retrieves a transaction result error message by block ID and transaction ID.
func RetrieveTransactionResultErrorMessage(blockID flow.Identifier, transactionID flow.Identifier, transactionResultErrorMessage *flow.TransactionResultErrorMessage) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransactionResultErrorMessage, blockID, transactionID), transactionResultErrorMessage)
}

// RetrieveTransactionResultErrorMessageByIndex retrieves a transaction result error message by block ID and index.
func RetrieveTransactionResultErrorMessageByIndex(blockID flow.Identifier, txIndex uint32, transactionResultErrorMessage *flow.TransactionResultErrorMessage) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransactionResultErrorMessageIndex, blockID, txIndex), transactionResultErrorMessage)
}

// TransactionResultErrorMessagesExists checks whether tx result error messages exist in the database.
func TransactionResultErrorMessagesExists(blockID flow.Identifier, blockExists *bool) func(*badger.Txn) error {
	return exists(makePrefix(codeTransactionResultErrorMessageIndex, blockID), blockExists)
}

// LookupTransactionResultErrorMessagesByBlockIDUsingIndex retrieves all tx result error messages for a block, by using
// tx_index index. This correctly handles cases of duplicate transactions within block.
func LookupTransactionResultErrorMessagesByBlockIDUsingIndex(blockID flow.Identifier, txResultErrorMessages *[]flow.TransactionResultErrorMessage) func(*badger.Txn) error {
	txErrIterFunc := func() (checkFunc, createFunc, handleFunc) {
		check := func(_ []byte) bool {
			return true
		}
		var val flow.TransactionResultErrorMessage
		create := func() interface{} {
			return &val
		}
		handle := func() error {
			*txResultErrorMessages = append(*txResultErrorMessages, val)
			return nil
		}
		return check, create, handle
	}

	return traverse(makePrefix(codeTransactionResultErrorMessageIndex, blockID), txErrIterFunc)
}
