// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"
)

func InsertBoundary(number uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeBoundary), number)
}

func UpdateBoundary(number uint64) func(*badger.Txn) error {
	return update(makePrefix(codeBoundary), number)
}

func RetrieveBoundary(number *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBoundary), number)
}

func InsertSealedBoundary(number uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeSealBoundary), number)
}

func UpdateSealedBoundary(number uint64) func(*badger.Txn) error {
	return update(makePrefix(codeSealBoundary), number)
}

func RetrieveSealedBoundary(number *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeSealBoundary), number)
}
