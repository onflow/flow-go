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
// layer (see storage.DKGKeys).
func InsertMyBeaconPrivateKey(epochCounter uint64, info *encodable.RandomBeaconPrivKey) func(*badger.Txn) error {
	return insert(makePrefix(codeBeaconPrivateKey, epochCounter), info)
}

// RetrieveMyBeaconPrivateKey retrieves the random beacon private key for the given epoch.
//
// CAUTION: This method stores confidential information and should only be
// used in the context of the secrets database. This is enforced in the above
// layer (see storage.DKGKeys).
func RetrieveMyBeaconPrivateKey(epochCounter uint64, info *encodable.RandomBeaconPrivKey) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBeaconPrivateKey, epochCounter), info)
}

// InsertDKGStartedForEpoch stores a flag indicating that the DKG has been started for
// the given epoch.
func InsertDKGStartedForEpoch(epochCounter uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeDKGStarted, epochCounter), true)
}

// RetrieveDKGStartedForEpoch retrieves the DKG started flag for the given epoch.
// If no flag is set, started is set to false.
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
func InsertDKGEndStateForEpoch(epochCounter uint64, endState flow.EndState) func(*badger.Txn) error {
	return insert(makePrefix(codeDKGEnded, epochCounter), endState)
}

// RetrieveDKGEndStateForEpoch retrieves the DKG end state for the epoch.
func RetrieveDKGEndStateForEpoch(epochCounter uint64, endState *flow.EndState) func(*badger.Txn) error {
	return retrieve(makePrefix(codeDKGEnded, epochCounter), endState)
}
