// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertTransactionError(blockID flow.Identifier, transactionError *flow.TransactionError) func(*badger.Txn) error {
	return insert(makePrefix(codeTransactionError, blockID, transactionError.TransactionID), transactionError)
}

func RetrieveTransactionError(blockID flow.Identifier, transactionID flow.Identifier, transactionError *flow.TransactionError) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransactionError, blockID, transactionID), transactionError)
}

func LookupTransactionErrorsByBlockID(blockID flow.Identifier, txErrors *[]flow.TransactionError) func(*badger.Txn) error {

	txErrIterFunc := func() (checkFunc, createFunc, handleFunc) {
		check := func(_ []byte) bool {
			return true
		}
		var val flow.TransactionError
		create := func() interface{} {
			return &val
		}
		handle := func() error {
			*txErrors = append(*txErrors, val)
			return nil
		}
		return check, create, handle
	}

	return traverse(makePrefix(codeEvent, blockID), txErrIterFunc)
}
