package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// DKGState stores state information about in-progress and completed DKGs, including
// computed keys. Must be instantiated using secrets database.
type DKGState struct {
	db       *badger.DB
	keyCache *Cache
}

// NewDKGState returns the DKGState implementation backed by Badger DB.
func NewDKGState(collector module.CacheMetrics, db *badger.DB) (*DKGState, error) {
	err := operation.EnsureSecretDB(db)
	if err != nil {
		return nil, fmt.Errorf("cannot instantiate dkg state storage in non-secret db: %w", err)
	}

	storeKey := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		epochCounter := key.(uint64)
		info := val.(*encodable.RandomBeaconPrivKey)
		return transaction.WithTx(operation.InsertMyBeaconPrivateKey(epochCounter, info))
	}

	retrieveKey := func(key interface{}) func(*badger.Txn) (interface{}, error) {
		epochCounter := key.(uint64)
		var info encodable.RandomBeaconPrivKey
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveMyBeaconPrivateKey(epochCounter, &info)(tx)
			return &info, err
		}
	}

	cache := newCache(collector, metrics.ResourceBeaconKey,
		withLimit(10),
		withStore(storeKey),
		withRetrieve(retrieveKey),
	)

	dkgState := &DKGState{
		db:       db,
		keyCache: cache,
	}

	return dkgState, nil
}

func (ds *DKGState) storeKeyTx(epochCounter uint64, key *encodable.RandomBeaconPrivKey) func(tx *transaction.Tx) error {
	return ds.keyCache.PutTx(epochCounter, key)
}

func (ds *DKGState) retrieveKeyTx(epochCounter uint64) func(tx *badger.Txn) (*encodable.RandomBeaconPrivKey, error) {
	return func(tx *badger.Txn) (*encodable.RandomBeaconPrivKey, error) {
		val, err := ds.keyCache.Get(epochCounter)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*encodable.RandomBeaconPrivKey), nil
	}
}

// InsertMyBeaconPrivateKey stores the random beacon private key for an epoch.
//
// CAUTION: these keys are stored before they are validated against the
// canonical key vector and may not be valid for use in signing. Use SafeBeaconKeys
// to guarantee only keys safe for signing are returned
func (ds *DKGState) InsertMyBeaconPrivateKey(epochCounter uint64, key crypto.PrivateKey) error {
	encodableKey := &encodable.RandomBeaconPrivKey{PrivateKey: key}
	return operation.RetryOnConflictTx(ds.db, transaction.Update, ds.storeKeyTx(epochCounter, encodableKey))
}

// RetrieveMyBeaconPrivateKey retrieves the random beacon private key for an epoch.
//
// CAUTION: these keys are stored before they are validated against the
// canonical key vector and may not be valid for use in signing. Use SafeBeaconKeys
// to guarantee only keys safe for signing are returned
func (ds *DKGState) RetrieveMyBeaconPrivateKey(epochCounter uint64) (crypto.PrivateKey, error) {
	tx := ds.db.NewTransaction(false)
	defer tx.Discard()
	encodableKey, err := ds.retrieveKeyTx(epochCounter)(tx)
	if err != nil {
		return nil, err
	}
	return encodableKey.PrivateKey, nil
}

// SetDKGStarted sets the flag indicating the DKG has started for the given epoch.
func (ds *DKGState) SetDKGStarted(epochCounter uint64) error {
	return ds.db.Update(operation.InsertDKGStartedForEpoch(epochCounter))
}

// GetDKGStarted checks whether the DKG has been started for the given epoch.
func (ds *DKGState) GetDKGStarted(epochCounter uint64) (bool, error) {
	var started bool
	err := ds.db.View(operation.RetrieveDKGStartedForEpoch(epochCounter, &started))
	return started, err
}

// SetDKGEndState stores that the DKG has ended, and its end state.
func (ds *DKGState) SetDKGEndState(epochCounter uint64, endState flow.DKGEndState) error {
	return ds.db.Update(operation.InsertDKGEndStateForEpoch(epochCounter, endState))
}

// SafeBeaconPrivateKeys is the safe beacon key storage backed by Badger DB.
type SafeBeaconPrivateKeys struct {
	state *DKGState
}

// NewSafeBeaconPrivateKeys returns a safe beacon key storage backed by Badger DB.
func NewSafeBeaconPrivateKeys(state *DKGState) *SafeBeaconPrivateKeys {
	return &SafeBeaconPrivateKeys{state: state}
}

// RetrieveMyBeaconPrivateKey retrieves my beacon private key for the given
// epoch, only if my key has been confirmed valid and safe for use.
//
// Returns:
// * (key, true, nil) if the key is present and confirmed valid
// * (nil, false, nil) if the key has been marked invalid (by SetDKGEnded)
// * (nil, false, error) for any other condition, or exception
func (keys *SafeBeaconPrivateKeys) RetrieveMyBeaconPrivateKey(epochCounter uint64) (key crypto.PrivateKey, safe bool, err error) {
	err = keys.state.db.View(func(txn *badger.Txn) error {

		// retrieve the key, error on any storage error
		var encodableKey *encodable.RandomBeaconPrivKey
		encodableKey, err = keys.state.retrieveKeyTx(epochCounter)(txn)
		if err != nil {
			key = nil
			safe = false
			return err
		}

		// retrieve the end state, error on any storage error (including not found)
		var endState flow.DKGEndState
		err = operation.RetrieveDKGEndStateForEpoch(epochCounter, &endState)(txn)
		if err != nil {
			key = nil
			safe = false
			return err
		}

		// for any end state besides success, return error
		if endState != flow.DKGEndStateSuccess {
			key = nil
			safe = false
			return fmt.Errorf("retrieving beacon for unsuccessful dkg run (dkg end state: %s)", endState)
		}

		// return the key only for successful end state
		safe = true
		key = encodableKey.PrivateKey
		return nil
	})
	return
}
