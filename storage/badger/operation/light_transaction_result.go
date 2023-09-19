// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

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
// tx_index index. This correctly handles cases of duplicate transactions within block, and should
// eventually replace uses of LookupTransactionResultsByBlockID
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
