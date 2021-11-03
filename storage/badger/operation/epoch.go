package operation

import (
	"errors"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
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

// InsertEpochEmergencyFallbackTriggered sets a flag in the DB indicating that
// epoch emergency fallback has been triggered. Calling this function multiple
// is a no-op and returns no expected errors.
func InsertEpochEmergencyFallbackTriggered() func(txn *badger.Txn) error {
	return SkipDuplicates(insert(makePrefix(codeEpochEmergencyFallbackTriggered), true))
}

// RetrieveEpochEmergencyFallbackTriggered retrieves the value of the flag
// indicating whether epoch emergency fallback has been triggered. If the key
// is not set, this results in triggered being set to false.
func RetrieveEpochEmergencyFallbackTriggered(triggered *bool) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		err := retrieve(makePrefix(codeEpochEmergencyFallbackTriggered), &triggered)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			*triggered = false
			return nil
		}
		return err
	}
}
