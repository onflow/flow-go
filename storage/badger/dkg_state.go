package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// RecoverablePrivateBeaconKeyState stores state information about in-progress and completed DKGs, including
// computed keys. Must be instantiated using secrets database.
// RecoverablePrivateBeaconKeyState is a specific module that allows to overwrite the beacon private key for a given epoch.
// This module is used *ONLY* in the epoch recovery process and only by the consensus participants.
// Each consensus participant takes part in the DKG, and after successfully finishing the DKG protocol it obtains a
// random beacon private key, which is stored in the database along with DKG end state `flow.DKGEndStateSuccess`.
// If for any reason the DKG fails, then the private key will be nil and DKG end state will be `flow.DKGEndStateDKGFailure`.
// When the epoch recovery takes place, we need to query the last valid beacon private key for the current replica and
// also set it for use during the Recovery Epoch, otherwise replicas won't be able to vote for blocks during the Recovery Epoch.
type RecoverablePrivateBeaconKeyState struct {
	db       *badger.DB
	keyCache *Cache[uint64, *encodable.RandomBeaconPrivKey]
}

var _ storage.EpochRecoveryMyBeaconKey = (*RecoverablePrivateBeaconKeyState)(nil)

// NewDKGState returns the RecoverablePrivateBeaconKeyState implementation backed by Badger DB.
func NewDKGState(collector module.CacheMetrics, db *badger.DB) (*RecoverablePrivateBeaconKeyState, error) {
	err := operation.EnsureSecretDB(db)
	if err != nil {
		return nil, fmt.Errorf("cannot instantiate dkg state storage in non-secret db: %w", err)
	}

	storeKey := func(epochCounter uint64, info *encodable.RandomBeaconPrivKey) func(*transaction.Tx) error {
		return transaction.WithTx(operation.InsertMyBeaconPrivateKey(epochCounter, info))
	}

	retrieveKey := func(epochCounter uint64) func(*badger.Txn) (*encodable.RandomBeaconPrivKey, error) {
		return func(tx *badger.Txn) (*encodable.RandomBeaconPrivKey, error) {
			var info encodable.RandomBeaconPrivKey
			err := operation.RetrieveMyBeaconPrivateKey(epochCounter, &info)(tx)
			return &info, err
		}
	}

	cache := newCache[uint64, *encodable.RandomBeaconPrivKey](collector, metrics.ResourceBeaconKey,
		withLimit[uint64, *encodable.RandomBeaconPrivKey](10),
		withStore(storeKey),
		withRetrieve(retrieveKey),
	)

	dkgState := &RecoverablePrivateBeaconKeyState{
		db:       db,
		keyCache: cache,
	}

	return dkgState, nil
}

func (ds *RecoverablePrivateBeaconKeyState) storeKeyTx(epochCounter uint64, key *encodable.RandomBeaconPrivKey) func(tx *transaction.Tx) error {
	return ds.keyCache.PutTx(epochCounter, key)
}

func (ds *RecoverablePrivateBeaconKeyState) retrieveKeyTx(epochCounter uint64) func(tx *badger.Txn) (*encodable.RandomBeaconPrivKey, error) {
	return func(tx *badger.Txn) (*encodable.RandomBeaconPrivKey, error) {
		val, err := ds.keyCache.Get(epochCounter)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

// InsertMyBeaconPrivateKey stores the random beacon private key for an epoch.
//
// CAUTION: these keys are stored before they are validated against the
// canonical key vector and may not be valid for use in signing. Use SafeBeaconKeys
// to guarantee only keys safe for signing are returned
func (ds *RecoverablePrivateBeaconKeyState) InsertMyBeaconPrivateKey(epochCounter uint64, key crypto.PrivateKey) error {
	if key == nil {
		return fmt.Errorf("will not store nil beacon key")
	}
	encodableKey := &encodable.RandomBeaconPrivKey{PrivateKey: key}
	return operation.RetryOnConflictTx(ds.db, transaction.Update, ds.storeKeyTx(epochCounter, encodableKey))
}

// UnsafeRetrieveMyBeaconPrivateKey retrieves the random beacon private key for an epoch.
//
// CAUTION: these keys are stored before they are validated against the
// canonical key vector and may not be valid for use in signing. Use SafeBeaconKeys
// to guarantee only keys safe for signing are returned
func (ds *RecoverablePrivateBeaconKeyState) UnsafeRetrieveMyBeaconPrivateKey(epochCounter uint64) (crypto.PrivateKey, error) {
	tx := ds.db.NewTransaction(false)
	defer tx.Discard()
	encodableKey, err := ds.retrieveKeyTx(epochCounter)(tx)
	if err != nil {
		return nil, err
	}
	return encodableKey.PrivateKey, nil
}

// SetDKGStarted sets the flag indicating the DKG has started for the given epoch.
func (ds *RecoverablePrivateBeaconKeyState) SetDKGStarted(epochCounter uint64) error {
	return ds.db.Update(operation.InsertDKGStartedForEpoch(epochCounter))
}

// GetDKGStarted checks whether the DKG has been started for the given epoch.
func (ds *RecoverablePrivateBeaconKeyState) GetDKGStarted(epochCounter uint64) (bool, error) {
	var started bool
	err := ds.db.View(operation.RetrieveDKGStartedForEpoch(epochCounter, &started))
	return started, err
}

// SetDKGEndState stores that the DKG has ended, and its end state.
func (ds *RecoverablePrivateBeaconKeyState) SetDKGEndState(epochCounter uint64, endState flow.DKGEndState) error {
	return ds.db.Update(operation.InsertDKGEndStateForEpoch(epochCounter, endState))
}

// GetDKGEndState retrieves the DKG end state for the epoch.
func (ds *RecoverablePrivateBeaconKeyState) GetDKGEndState(epochCounter uint64) (flow.DKGEndState, error) {
	var endState flow.DKGEndState
	err := ds.db.Update(operation.RetrieveDKGEndStateForEpoch(epochCounter, &endState))
	return endState, err
}

// RetrieveMyBeaconPrivateKey retrieves my beacon private key for the given
// epoch, only if my key has been confirmed valid and safe for use.
//
// Returns:
//   - (key, true, nil) if the key is present and confirmed valid
//   - (nil, false, nil) if the key has been marked invalid or unavailable
//     -> no beacon key will ever be available for the epoch in this case
//   - (nil, false, storage.ErrNotFound) if the DKG has not ended
//   - (nil, false, error) for any unexpected exception
func (ds *RecoverablePrivateBeaconKeyState) RetrieveMyBeaconPrivateKey(epochCounter uint64) (key crypto.PrivateKey, safe bool, err error) {
	err = ds.db.View(func(txn *badger.Txn) error {

		// retrieve the end state
		var endState flow.DKGEndState
		err = operation.RetrieveDKGEndStateForEpoch(epochCounter, &endState)(txn)
		if err != nil {
			key = nil
			safe = false
			return err // storage.ErrNotFound or exception
		}

		// for any end state besides success and recovery, the key is not safe
		if endState == flow.DKGEndStateSuccess || endState == flow.RandomBeaconKeyRecovered {
			// retrieve the key - any storage error (including `storage.ErrNotFound`) is an exception
			var encodableKey *encodable.RandomBeaconPrivKey
			encodableKey, err = ds.retrieveKeyTx(epochCounter)(txn)
			if err != nil {
				key = nil
				safe = false
				return irrecoverable.NewExceptionf("could not retrieve beacon key for epoch %d with successful DKG: %v", epochCounter, err)
			}

			// return the key only for successful end state
			safe = true
			key = encodableKey.PrivateKey
		} else {
			key = nil
			safe = false
		}

		return nil
	})
	return
}

// UpsertMyBeaconPrivateKey overwrites the random beacon private key for the epoch that recovers the protocol from
// Epoch Fallback Mode. Effectively, this function overwrites whatever might be available in the database with
// the given private key and sets the DKGEndState to `flow.DKGEndStateRecovered`.
// No errors are expected during normal operations.
func (ds *RecoverablePrivateBeaconKeyState) UpsertMyBeaconPrivateKey(epochCounter uint64, key crypto.PrivateKey) error {
	if key == nil {
		return fmt.Errorf("will not store nil beacon key")
	}
	encodableKey := &encodable.RandomBeaconPrivKey{PrivateKey: key}
	err := ds.db.Update(func(txn *badger.Txn) error {
		err := operation.UpsertMyBeaconPrivateKey(epochCounter, encodableKey)(txn)
		if err != nil {
			return err
		}
		return operation.UpsertDKGEndStateForEpoch(epochCounter, flow.RandomBeaconKeyRecovered)(txn)
	})
	if err != nil {
		return fmt.Errorf("could not overwrite beacon key for epoch %d: %w", epochCounter, err)
	}
	ds.keyCache.Insert(epochCounter, encodableKey)
	return nil
}
