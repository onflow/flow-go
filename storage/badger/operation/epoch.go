package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

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

func InsertEpochStatus(blockID flow.Identifier, status *flow.EpochStatus) func(*badger.Txn) error {
	return insert(makePrefix(codeBlockEpochStatus, blockID), status)
}

func RetrieveEpochStatus(blockID flow.Identifier, status *flow.EpochStatus) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBlockEpochStatus, blockID), status)
}
