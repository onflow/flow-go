package operation

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

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

// MigrateDKGEndStateFromV1 migrates the database that was used in protocol version v1 to the v2.
// It reads already stored data by deprecated prefix and writes it to the new prefix with values converted to the new representation.
// TODO(EFM, #6794): This function is introduced to implement a backward-compatible upgrade from v1 to v2.
// Remove this once we complete the network upgrade.
func MigrateDKGEndStateFromV1(log zerolog.Logger) func(txn *badger.Txn) error {
	return func(txn *badger.Txn) error {
		var ops []func(*badger.Txn) error
		err := traverse(makePrefix(codeDKGEndState), func() (checkFunc, createFunc, handleFunc) {
			var epochCounter uint64
			check := func(key []byte) bool {
				epochCounter = binary.BigEndian.Uint64(key[1:]) // omit code
				return true
			}
			var oldState uint32
			create := func() interface{} {
				return &oldState
			}
			handle := func() error {
				newState := flow.DKGStateUninitialized
				switch oldState {
				case 0: // DKGEndStateUnknown
					newState = flow.DKGStateUninitialized
				case 1: // DKGEndStateSuccess
					newState = flow.RandomBeaconKeyCommitted
				case 2, 3, 4: // DKGEndStateInconsistentKey, DKGEndStateNoKey, DKGEndStateDKGFailure
					newState = flow.DKGStateFailure
				}

				// schedule upsert of the new state and removal of the old state
				// this will be executed after to split collecting and modifying of data.
				ops = append(ops,
					UpsertDKGStateForEpoch(epochCounter, newState),
					remove(makePrefix(codeDKGEndState, epochCounter)))
				log.Info().Msgf("removing %d->%d, writing %d->%d", epochCounter, oldState, epochCounter, newState)

				return nil
			}
			return check, create, handle
		})(txn)
		if err != nil {
			return fmt.Errorf("could not collect deprecated DKG end states: %w", err)
		}

		for _, op := range ops {
			if err := op(txn); err != nil {
				return fmt.Errorf("aborting conversion from DKG end states: %w", err)
			}
		}
		if len(ops) > 0 {
			log.Info().Msgf("finished migrating %d DKG end states", len(ops))
		}
		return nil
	}
}
