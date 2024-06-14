package operation

import (
	"errors"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InsertEpochSetup(eventID flow.Identifier, event *flow.EpochSetup) func(pebble.Writer) error {
	return insert(makePrefix(codeEpochSetup, eventID), event)
}

func RetrieveEpochSetup(eventID flow.Identifier, event *flow.EpochSetup) func(pebble.Reader) error {
	return retrieve(makePrefix(codeEpochSetup, eventID), event)
}

func InsertEpochCommit(eventID flow.Identifier, event *flow.EpochCommit) func(pebble.Writer) error {
	return insert(makePrefix(codeEpochCommit, eventID), event)
}

func RetrieveEpochCommit(eventID flow.Identifier, event *flow.EpochCommit) func(pebble.Reader) error {
	return retrieve(makePrefix(codeEpochCommit, eventID), event)
}

// SetEpochEmergencyFallbackTriggered sets a flag in the DB indicating that
// epoch emergency fallback has been triggered, and the block where it was triggered.
//
// EFM can be triggered in two ways:
//  1. Finalizing the first block past the epoch commitment deadline, when the
//     next epoch has not yet been committed (see protocol.Params for more detail)
//  2. Finalizing a fork in which an invalid service event was incorporated.
//
// TODO: in pebble/mutator.go must implement RetrieveEpochEmergencyFallbackTriggeredBlockID and
// verify not exist
// Note: The caller needs to ensure a previous value was not stored
func SetEpochEmergencyFallbackTriggered(blockID flow.Identifier) func(txn pebble.Writer) error {
	return insert(makePrefix(codeEpochEmergencyFallbackTriggered), blockID)
}

// RetrieveEpochEmergencyFallbackTriggeredBlockID gets the block ID where epoch
// emergency was triggered.
func RetrieveEpochEmergencyFallbackTriggeredBlockID(blockID *flow.Identifier) func(pebble.Reader) error {
	return retrieve(makePrefix(codeEpochEmergencyFallbackTriggered), blockID)
}

// CheckEpochEmergencyFallbackTriggered retrieves the value of the flag
// indicating whether epoch emergency fallback has been triggered. If the key
// is not set, this results in triggered being set to false.
func CheckEpochEmergencyFallbackTriggered(triggered *bool) func(pebble.Reader) error {
	return func(tx pebble.Reader) error {
		var blockID flow.Identifier
		err := RetrieveEpochEmergencyFallbackTriggeredBlockID(&blockID)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			// flag unset, EFM not triggered
			*triggered = false
			return nil
		} else if err != nil {
			// storage error, set triggered to zero value
			*triggered = false
			return err
		}
		// flag is set, EFM triggered
		*triggered = true
		return err
	}
}
