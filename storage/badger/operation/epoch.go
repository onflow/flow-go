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

// SetEpochEmergencyFallbackTriggered sets a flag in the DB indicating that
// epoch emergency fallback has been triggered, and the block where it was triggered.
// EECC can be triggered by 2 blocks:
//
// 1. The first block of a new epoch, when that epoch has not been set up.
// 2. The block where an invalid service event is being applied to the state.
//
// Calling this function multiple times is a no-op and returns no expected errors.
func SetEpochEmergencyFallbackTriggered(blockID flow.Identifier) func(txn *badger.Txn) error {
	return SkipDuplicates(insert(makePrefix(codeEpochEmergencyFallbackTriggered), blockID))
}

// RetrieveEpochEmergencyFallbackTriggeredBlockID gets the block ID where epoch
// emergency was triggered.
func RetrieveEpochEmergencyFallbackTriggeredBlockID(blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochEmergencyFallbackTriggered), blockID)
}

// CheckEpochEmergencyFallbackTriggered retrieves the value of the flag
// indicating whether epoch emergency fallback has been triggered. If the key
// is not set, this results in triggered being set to false.
func CheckEpochEmergencyFallbackTriggered(triggered *bool) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		var blockID flow.Identifier
		err := RetrieveEpochEmergencyFallbackTriggeredBlockID(&blockID)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			// flag unset, EECC not triggered
			*triggered = false
			return nil
		} else if err != nil {
			// storage error, set triggered to zero value
			*triggered = false
			return err
		}
		// flag is set, EECC triggered
		*triggered = true
		return err
	}
}
