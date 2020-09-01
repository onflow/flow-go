package operation

import (
	"github.com/dapperlabs/flow-go/model/flow"

	"github.com/dgraph-io/badger/v2"
)

func InsertEpochCounter(counter uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochCounter), counter)
}

func UpdateEpochCounter(counter uint64) func(*badger.Txn) error {
	return update(makePrefix(codeEpochCounter), counter)
}

func RetrieveEpochCounter(counter *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochCounter), counter)
}

func InsertEpochHeight(counter uint64, height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochHeight, counter), height)
}

func UpdateEpochHeight(counter uint64, height uint64) func(*badger.Txn) error {
	return update(makePrefix(codeEpochHeight, counter), height)
}

func RetrieveEpochHeight(counter uint64, height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochHeight, counter), height)
}

func IndexEpochStart(counter uint64, view uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochStart, counter), view)
}

func LookupEpochStart(counter uint64, view *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochStart, counter), view)
}

func InsertEpochSetup(counter uint64, event *flow.EpochSetup) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochSetup, counter), event)
}

func RetrieveEpochSetup(counter uint64, event *flow.EpochSetup) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochSetup, counter), event)
}

func InsertEpochCommit(counter uint64, event *flow.EpochCommit) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochCommit, counter), event)
}

func RetrieveEpochCommit(counter uint64, event *flow.EpochCommit) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochCommit, counter), event)
}
