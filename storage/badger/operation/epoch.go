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

func InsertEpochSetup(eventID flow.Identifier, event *flow.EpochSetup) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochSetup, eventID), event)
}

func RetrieveEpochSetup(eventID flow.Identifier, event *flow.EpochSetup) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochSetup, eventID), event)
}

func InsertEpochCommit(eventID flow.Identifier, event *flow.EpochCommit) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochCommit, eventID), event)
}

func RetrieveEpochCommit(eventID flow.Identifier, event *flow.EpochCommit) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochCommit, eventID), event)
}

func InsertEpochState(blockID flow.Identifier, state *flow.EpochState) func(*badger.Txn) error {
	return insert(makePrefix(codeBlockEpochPreparation, blockID), state)
}

func RetrieveEpochState(blockID flow.Identifier, state *flow.EpochState) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBlockEpochPreparation, blockID), state)
}
