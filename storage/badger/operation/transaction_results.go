// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertTransactionResult(blockID flow.Identifier, transactionResult *flow.TransactionResult) func(*badger.Txn) error {
	return insert(makePrefix(codeTransactionResult, blockID, transactionResult.TransactionID), transactionResult)
}

func RetrieveTransactionResult(blockID flow.Identifier, transactionID flow.Identifier, transactionResult *flow.TransactionResult) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransactionResult, blockID, transactionID), transactionResult)
}

func LookupTransactionResultsByBlockID(blockID flow.Identifier, txResults *[]flow.TransactionResult) func(*badger.Txn) error {

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

	return traverse(makePrefix(codeEvent, blockID), txErrIterFunc)
}
