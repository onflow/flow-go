package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

// DKGState stores state information about in-progress and completed DKGs, including
// computed keys. Must be instantiated using secrets database.
type DKGState struct {
	db       *pebble.DB
	keyCache *Cache[uint64, *encodable.RandomBeaconPrivKey]
}

// NewDKGState returns the DKGState implementation backed by pebble DB.
func NewDKGState(collector module.CacheMetrics, db *pebble.DB) (*DKGState, error) {
	err := operation.EnsureSecretDB(db)
	if err != nil {
		return nil, fmt.Errorf("cannot instantiate dkg state storage in non-secret db: %w", err)
	}

	storeKey := operation.InsertMyBeaconPrivateKey

	retrieveKey := func(epochCounter uint64) func(pebble.Reader) (*encodable.RandomBeaconPrivKey, error) {
		return func(tx pebble.Reader) (*encodable.RandomBeaconPrivKey, error) {
			var info encodable.RandomBeaconPrivKey
			err := operation.RetrieveMyBeaconPrivateKey(epochCounter, &info)(tx)
			return &info, err
		}
	}

	cache := newCache(collector, metrics.ResourceBeaconKey,
		withLimit[uint64, *encodable.RandomBeaconPrivKey](10),
		withStore(storeKey),
		withRetrieve(retrieveKey),
	)

	dkgState := &DKGState{
		db:       db,
		keyCache: cache,
	}

	return dkgState, nil
}

func (ds *DKGState) storeKeyTx(epochCounter uint64, key *encodable.RandomBeaconPrivKey) func(pebble.Writer) error {
	return ds.keyCache.PutTx(epochCounter, key)
}

func (ds *DKGState) retrieveKeyTx(epochCounter uint64) func(tx pebble.Reader) (*encodable.RandomBeaconPrivKey, error) {
	return func(tx pebble.Reader) (*encodable.RandomBeaconPrivKey, error) {
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
func (ds *DKGState) InsertMyBeaconPrivateKey(epochCounter uint64, key crypto.PrivateKey) error {
	if key == nil {
		return fmt.Errorf("will not store nil beacon key")
	}
	encodableKey := &encodable.RandomBeaconPrivKey{PrivateKey: key}
	return ds.storeKeyTx(epochCounter, encodableKey)(ds.db)
}

// RetrieveMyBeaconPrivateKey retrieves the random beacon private key for an epoch.
//
// CAUTION: these keys are stored before they are validated against the
// canonical key vector and may not be valid for use in signing. Use SafeBeaconKeys
// to guarantee only keys safe for signing are returned
func (ds *DKGState) RetrieveMyBeaconPrivateKey(epochCounter uint64) (crypto.PrivateKey, error) {
	encodableKey, err := ds.retrieveKeyTx(epochCounter)(ds.db)
	if err != nil {
		return nil, err
	}
	return encodableKey.PrivateKey, nil
}

// SetDKGStarted sets the flag indicating the DKG has started for the given epoch.
func (ds *DKGState) SetDKGStarted(epochCounter uint64) error {
	return operation.InsertDKGStartedForEpoch(epochCounter)(ds.db)
}

// GetDKGStarted checks whether the DKG has been started for the given epoch.
func (ds *DKGState) GetDKGStarted(epochCounter uint64) (bool, error) {
	var started bool
	err := operation.RetrieveDKGStartedForEpoch(epochCounter, &started)(ds.db)
	return started, err
}

// SetDKGEndState stores that the DKG has ended, and its end state.
func (ds *DKGState) SetDKGEndState(epochCounter uint64, endState flow.DKGEndState) error {
	return operation.InsertDKGEndStateForEpoch(epochCounter, endState)(ds.db)
}

// GetDKGEndState retrieves the DKG end state for the epoch.
func (ds *DKGState) GetDKGEndState(epochCounter uint64) (flow.DKGEndState, error) {
	var endState flow.DKGEndState
	err := operation.RetrieveDKGEndStateForEpoch(epochCounter, &endState)(ds.db)
	return endState, err
}

// SafeBeaconPrivateKeys is the safe beacon key storage backed by pebble DB.
type SafeBeaconPrivateKeys struct {
	state *DKGState
}

// NewSafeBeaconPrivateKeys returns a safe beacon key storage backed by pebble DB.
func NewSafeBeaconPrivateKeys(state *DKGState) *SafeBeaconPrivateKeys {
	return &SafeBeaconPrivateKeys{state: state}
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
func (keys *SafeBeaconPrivateKeys) RetrieveMyBeaconPrivateKey(epochCounter uint64) (key crypto.PrivateKey, safe bool, err error) {
	// retrieve the end state
	var endState flow.DKGEndState
	err = operation.RetrieveDKGEndStateForEpoch(epochCounter, &endState)(keys.state.db)
	if err != nil {
		return nil, false, err
	}

	// for any end state besides success, the key is not safe
	if endState != flow.DKGEndStateSuccess {
		return nil, false, nil
	}

	// retrieve the key - any storage error (including not found) is an exception
	var encodableKey *encodable.RandomBeaconPrivKey
	encodableKey, err = keys.state.retrieveKeyTx(epochCounter)(keys.state.db)
	if err != nil {
		return nil, false, fmt.Errorf("[unexpected] could not retrieve beacon key for epoch %d with successful DKG: %v", epochCounter, err)
	}

	// return the key only for successful end state
	return encodableKey.PrivateKey, true, nil
}
