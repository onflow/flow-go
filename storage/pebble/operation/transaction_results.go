package operation

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InsertTransactionResult(blockID flow.Identifier, transactionResult *flow.TransactionResult) func(pebble.Writer) error {
	return insert(makePrefix(codeTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func BatchIndexTransactionResult(blockID flow.Identifier, txIndex uint32, transactionResult *flow.TransactionResult) func(storage.Transaction) error {
	return func(batch storage.Transaction) error {
		return insert(makePrefix(codeTransactionResultIndex, blockID, txIndex), transactionResult)(NewBatchWriter(batch))
	}
}

func RetrieveTransactionResult(blockID flow.Identifier, transactionID flow.Identifier, transactionResult *flow.TransactionResult) func(pebble.Reader) error {
	return retrieve(makePrefix(codeTransactionResult, blockID, transactionID), transactionResult)
}
func RetrieveTransactionResultByIndex(blockID flow.Identifier, txIndex uint32, transactionResult *flow.TransactionResult) func(pebble.Reader) error {
	return retrieve(makePrefix(codeTransactionResultIndex, blockID, txIndex), transactionResult)
}

// LookupTransactionResultsByBlockIDUsingIndex retrieves all tx results for a block, by using
// tx_index index. This correctly handles cases of duplicate transactions within block.
func LookupTransactionResultsByBlockIDUsingIndex(blockID flow.Identifier, txResults *[]flow.TransactionResult) func(pebble.Reader) error {

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
func RemoveTransactionResultsByBlockID(blockID flow.Identifier) func(pebble.Writer) error {
	return func(txn pebble.Writer) error {

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
// if pebble fails to process request
func BatchRemoveTransactionResultsByBlockID(blockID flow.Identifier, batch storage.Transaction) func(pebble.Writer) error {
	return func(txn pebble.Writer) error {
		prefix := makePrefix(codeTransactionResult, blockID)
		err := removeByPrefix(prefix)(txn)
		if err != nil {
			return fmt.Errorf("could not remove transaction results for block %v: %w", blockID, err)
		}

		return nil
	}
}

func InsertLightTransactionResult(blockID flow.Identifier, transactionResult *flow.LightTransactionResult) func(pebble.Writer) error {
	return insert(makePrefix(codeLightTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func BatchIndexLightTransactionResult(blockID flow.Identifier, txIndex uint32, transactionResult *flow.LightTransactionResult) func(batch storage.Transaction) error {
	return func(batch storage.Transaction) error {
		return insert(makePrefix(codeLightTransactionResultIndex, blockID, txIndex), transactionResult)(NewBatchWriter(batch))
	}
}

func RetrieveLightTransactionResult(blockID flow.Identifier, transactionID flow.Identifier, transactionResult *flow.LightTransactionResult) func(pebble.Reader) error {
	return retrieve(makePrefix(codeLightTransactionResult, blockID, transactionID), transactionResult)
}

func RetrieveLightTransactionResultByIndex(blockID flow.Identifier, txIndex uint32, transactionResult *flow.LightTransactionResult) func(pebble.Reader) error {
	return retrieve(makePrefix(codeLightTransactionResultIndex, blockID, txIndex), transactionResult)
}

// LookupLightTransactionResultsByBlockIDUsingIndex retrieves all tx results for a block, but using
// tx_index index. This correctly handles cases of duplicate transactions within block.
func LookupLightTransactionResultsByBlockIDUsingIndex(blockID flow.Identifier, txResults *[]flow.LightTransactionResult) func(pebble.Reader) error {

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
