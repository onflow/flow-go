// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"
)

func InsertRootHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeFinalizedRootHeight), height)
}

func RetrieveRootHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeFinalizedRootHeight), height)
}

func InsertSealedRootHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeSealedRootHeight), height)
}

func RetrieveSealedRootHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeSealedRootHeight), height)
}

func InsertFinalizedHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeFinalizedHeight), height)
}

func UpdateFinalizedHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(codeFinalizedHeight), height)
}

func RetrieveFinalizedHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeFinalizedHeight), height)
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

// InsertLastCompleteBlockHeightIfNotExists inserts the last full block height if it is not already set.
// Calling this function multiple times is a no-op and returns no expected errors.
func InsertLastCompleteBlockHeightIfNotExists(height uint64) func(*badger.Txn) error {
	return SkipDuplicates(InsertLastCompleteBlockHeight(height))
}

func InsertLastCompleteBlockHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeLastCompleteBlockHeight), height)
}

func UpdateLastCompleteBlockHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(codeLastCompleteBlockHeight), height)
}

func RetrieveLastCompleteBlockHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeLastCompleteBlockHeight), height)
}
