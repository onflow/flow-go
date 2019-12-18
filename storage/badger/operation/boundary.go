// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/storage"
)

func InsertNewBoundary(number uint64) func(*badger.Txn) storage.Error {
	return insertNew(makePrefix(codeBoundary), number)
}

func UpdateBoundary(number uint64) func(*badger.Txn) storage.Error {
	return update(makePrefix(codeBoundary), number)
}

func RetrieveBoundary(number *uint64) func(*badger.Txn) storage.Error {
	return retrieve(makePrefix(codeBoundary), number)
}
