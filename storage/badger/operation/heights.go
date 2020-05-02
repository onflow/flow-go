// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"
)

func InsertFinalizedHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeFinalizedHeight), height)
}

func UpdateFinalizedHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(codeFinalizedHeight), height)
}

func RetrieveFinalizedHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeFinalizedHeight), height)
}

func InsertExecutedHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutedHeight), height)
}

func UpdateExecutedHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(codeExecutedHeight), height)
}

func RetrieveExecutedHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutedHeight), height)
}

func InsertSealedHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeSealedHeight), height)
}

func UpdateSealedHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(codeSealedHeight), height)
}

func RetrieveSealedHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeSealedHeight), height)
}
