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
// Error returns: storage.ErrAlreadyExists
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

// InsertDKGStartedForEpoch stores a flag indicating that the DKG has been started for the given epoch.
// Returns: storage.ErrAlreadyExists
// Error returns: storage.ErrAlreadyExists
func InsertDKGStartedForEpoch(epochCounter uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeDKGStarted, epochCounter), true)
}

// RetrieveDKGStartedForEpoch retrieves the DKG started flag for the given epoch.
// If no flag is set, started is set to false and no error is returned.
// No errors expected during normal operation.
func RetrieveDKGStartedForEpoch(epochCounter uint64, started *bool) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		err := retrieve(makePrefix(codeDKGStarted, epochCounter), started)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			// flag not set - therefore DKG not started
			*started = false
			return nil
		} else if err != nil {
			// storage error - set started to zero value
			*started = false
			return err
		}
		return nil
	}
}

// InsertDKGEndStateForEpoch stores the DKG end state for the epoch.
// Error returns: storage.ErrAlreadyExists
func InsertDKGEndStateForEpoch(epochCounter uint64, endState flow.DKGEndState) func(*badger.Txn) error {
	return insert(makePrefix(codeDKGEnded, epochCounter), endState)
}

// UpsertDKGEndStateForEpoch stores the DKG end state for the epoch, irrespective of whether an entry for
// the given epoch counter already exists in the database or not.
// CAUTION: this method has to be used only in the very specific edge-cases of epoch recovery. For storing the
// DKG results obtained on the happy-path, please use method `InsertDKGEndStateForEpoch`.
func UpsertDKGEndStateForEpoch(epochCounter uint64, endState flow.DKGEndState) func(*badger.Txn) error {
	return upsert(makePrefix(codeDKGEnded, epochCounter), endState)
}

// RetrieveDKGEndStateForEpoch retrieves the DKG end state for the epoch.
// Error returns: storage.ErrNotFound
func RetrieveDKGEndStateForEpoch(epochCounter uint64, endState *flow.DKGEndState) func(*badger.Txn) error {
	return retrieve(makePrefix(codeDKGEnded, epochCounter), endState)
}
