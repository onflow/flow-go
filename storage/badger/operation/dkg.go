package operation

import (
	"errors"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertMyBeaconPrivateKey stores the random beacon private key for the given epoch.
//
// CAUTION: This method stores confidential information and should only be
// used in the context of the secrets database. This is enforced in the above
// layer (see storage.DKGState).
// Error returns: [storage.ErrAlreadyExists].
func InsertMyBeaconPrivateKey(epochCounter uint64, info *encodable.RandomBeaconPrivKey) func(*badger.Txn) error {
	return insert(makePrefix(codeBeaconPrivateKey, epochCounter), info)
}

// UpsertMyBeaconPrivateKey stores the random beacon private key, irrespective of whether an entry for
// the given epoch counter already exists in the database or not.
//
// CAUTION: This method stores confidential information and should only be
// used in the context of the secrets database. This is enforced in the above
// layer (see storage.EpochRecoveryMyBeaconKey).
// This method has to be used only in very specific cases, like epoch recovery, for normal usage use InsertMyBeaconPrivateKey.
func UpsertMyBeaconPrivateKey(epochCounter uint64, info *encodable.RandomBeaconPrivKey) func(*badger.Txn) error {
	return upsert(makePrefix(codeBeaconPrivateKey, epochCounter), info)
}

// RetrieveMyBeaconPrivateKey retrieves the random beacon private key for the given epoch.
//
// CAUTION: This method stores confidential information and should only be
// used in the context of the secrets database. This is enforced in the above
// layer (see storage.DKGState).
// Error returns: storage.ErrNotFound
func RetrieveMyBeaconPrivateKey(epochCounter uint64, info *encodable.RandomBeaconPrivKey) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBeaconPrivateKey, epochCounter), info)
}

// RetrieveDKGStartedForEpoch retrieves whether DKG has started for the given epoch.
// If no flag is set, started is set to false and no error is returned.
// No errors expected during normal operation.
func RetrieveDKGStartedForEpoch(epochCounter uint64, started *bool) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		var state flow.DKGState
		err := RetrieveDKGStateForEpoch(epochCounter, &state)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			// flag not set - therefore DKG not started
			*started = false
			return nil
		} else if err != nil {
			// storage error - set started to zero value
			*started = false
			return err
		} else {
			*started = true
		}
		return nil
	}
}

// UpsertDKGStateForEpoch stores the current state of Random Beacon Recoverable State Machine for the epoch, irrespective of whether an entry for
// the given epoch counter already exists in the database or not.
func UpsertDKGStateForEpoch(epochCounter uint64, newState flow.DKGState) func(*badger.Txn) error {
	return upsert(makePrefix(codeDKGState, epochCounter), newState)
}

// RetrieveDKGStateForEpoch retrieves the current DKG state for the epoch.
// Error returns: [storage.ErrNotFound]
func RetrieveDKGStateForEpoch(epochCounter uint64, currentState *flow.DKGState) func(*badger.Txn) error {
	return retrieve(makePrefix(codeDKGState, epochCounter), currentState)
}
