package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/crypto"
	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// allowedStateTransitions defines the allowed state transitions for the Recoverable Random Beacon State Machine.
var allowedStateTransitions = map[flow.DKGState][]flow.DKGState{
	flow.DKGStateStarted:          {flow.DKGStateCompleted, flow.DKGStateFailure, flow.RandomBeaconKeyCommitted},
	flow.DKGStateCompleted:        {flow.RandomBeaconKeyCommitted, flow.DKGStateFailure},
	flow.RandomBeaconKeyCommitted: {},
	flow.DKGStateFailure:          {flow.RandomBeaconKeyCommitted, flow.DKGStateFailure},
	flow.DKGStateUninitialized:    {flow.DKGStateStarted, flow.DKGStateFailure, flow.RandomBeaconKeyCommitted},
}

// RecoverablePrivateBeaconKeyStateMachine stores state information about in-progress and completed DKGs, including
// computed keys. Must be instantiated using secrets database. On the happy path, each consensus
// committee member takes part in the DKG, and after successfully finishing the DKG protocol it obtains a
// random beacon private key, which is stored in the database along with DKG state [flow.DKGStateCompleted].
// If for any reason the DKG fails, then the private key will be nil and DKG state is set to [flow.DKGStateFailure].
// When the epoch recovery takes place, we need to query the last valid beacon private key for the current replica and
// also set it for use during the Recovery Epoch, otherwise replicas won't be able to vote for blocks during the Recovery Epoch.
type RecoverablePrivateBeaconKeyStateMachine struct {
	db       *badger.DB
	keyCache *Cache[uint64, *encodable.RandomBeaconPrivKey]
}

var _ storage.EpochRecoveryMyBeaconKey = (*RecoverablePrivateBeaconKeyStateMachine)(nil)

// NewRecoverableRandomBeaconStateMachine returns the RecoverablePrivateBeaconKeyStateMachine implementation backed by Badger DB.
// No errors are expected during normal operations.
func NewRecoverableRandomBeaconStateMachine(collector module.CacheMetrics, db *badger.DB) (*RecoverablePrivateBeaconKeyStateMachine, error) {
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

	return &RecoverablePrivateBeaconKeyStateMachine{
		db:       db,
		keyCache: cache,
	}, nil
}

// InsertMyBeaconPrivateKey stores the random beacon private key for an epoch and transitions the
// state machine into the [flow.DKGStateCompleted] state.
//
// CAUTION: these keys are stored before they are validated against the
// canonical key vector and may not be valid for use in signing. Use [storage.SafeBeaconKeys]
// interface to guarantee only keys safe for signing are returned.
// Error returns:
//   - [storage.ErrAlreadyExists] - if there is already a key stored for given epoch.
//   - [storage.InvalidDKGStateTransitionError] - if the requested state transition is invalid.
func (ds *RecoverablePrivateBeaconKeyStateMachine) InsertMyBeaconPrivateKey(epochCounter uint64, key crypto.PrivateKey) error {
	if key == nil {
		return fmt.Errorf("will not store nil beacon key")
	}
	encodableKey := &encodable.RandomBeaconPrivKey{PrivateKey: key}
	return operation.RetryOnConflictTx(ds.db, transaction.Update, func(tx *transaction.Tx) error {
		currentState, err := retrieveCurrentStateTx(epochCounter)(tx.DBTxn)
		if err != nil {
			return err
		}
		err = ds.keyCache.PutTx(epochCounter, encodableKey)(tx)
		if err != nil {
			return err
		}
		return ds.processStateTransition(epochCounter, currentState, flow.DKGStateCompleted)(tx)
	})
}

// UnsafeRetrieveMyBeaconPrivateKey retrieves the random beacon private key for an epoch.
//
// CAUTION: these keys were stored before they are validated against the
// canonical key vector and may not be valid for use in signing. Use [storage.SafeBeaconKeys]
// interface to guarantee only keys safe for signing are returned
// Error returns:
//   - [storage.ErrNotFound] - if there is no key stored for given epoch.
func (ds *RecoverablePrivateBeaconKeyStateMachine) UnsafeRetrieveMyBeaconPrivateKey(epochCounter uint64) (crypto.PrivateKey, error) {
	tx := ds.db.NewTransaction(false)
	defer tx.Discard()
	encodableKey, err := ds.keyCache.Get(epochCounter)(tx)
	if err != nil {
		return nil, err
	}
	return encodableKey.PrivateKey, nil
}

// IsDKGStarted checks whether the DKG has been started for the given epoch.
// No errors expected during normal operation.
func (ds *RecoverablePrivateBeaconKeyStateMachine) IsDKGStarted(epochCounter uint64) (bool, error) {
	var started bool
	err := ds.db.View(operation.RetrieveDKGStartedForEpoch(epochCounter, &started))
	return started, err
}

// SetDKGState performs a state transition for the Random Beacon Recoverable State Machine.
// Some state transitions may not be possible using this method. For instance, we might not be able to enter [flow.DKGStateCompleted]
// state directly from [flow.DKGStateStarted], even if such transition is valid. The reason for this is that some states require additional
// data to be processed by the state machine before the transition can be made. For such cases there are dedicated methods that should be used, ex.
// InsertMyBeaconPrivateKey and UpsertMyBeaconPrivateKey, which allow to store the needed data and perform the transition in one atomic operation.
// Error returns:
//   - [storage.InvalidDKGStateTransitionError] - if the requested state transition is invalid.
func (ds *RecoverablePrivateBeaconKeyStateMachine) SetDKGState(epochCounter uint64, newState flow.DKGState) error {
	return operation.RetryOnConflictTx(ds.db, transaction.Update, func(tx *transaction.Tx) error {
		currentState, err := retrieveCurrentStateTx(epochCounter)(tx.DBTxn)
		if err != nil {
			return err
		}

		if newState == flow.RandomBeaconKeyCommitted {
			return storage.NewInvalidDKGStateTransitionErrorf(currentState, newState, "cannot transition directly to committed state without evidence")
		} else {
			return operation.RetryOnConflictTx(ds.db, transaction.Update, ds.processStateTransition(epochCounter, currentState, newState))
		}
	})
}

// Error returns:
//   - storage.InvalidDKGStateTransitionError - if the requested state transition is invalid
func (ds *RecoverablePrivateBeaconKeyStateMachine) processStateTransition(epochCounter uint64, currentState, newState flow.DKGState) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		allowedStates := allowedStateTransitions[currentState]
		if slices.Index(allowedStates, newState) < 0 {
			return storage.NewInvalidDKGStateTransitionErrorf(currentState, newState, "not allowed")
		}

		// ensure invariant holds and we still have a valid private key stored
		if newState == flow.RandomBeaconKeyCommitted || newState == flow.DKGStateCompleted {
			_, err := ds.keyCache.Get(epochCounter)(tx.DBTxn)
			if err != nil {
				return storage.NewInvalidDKGStateTransitionErrorf(currentState, newState, "cannot transition without a valid random beacon key: %w", err)
			}
		}

		return operation.UpsertDKGStateForEpoch(epochCounter, newState)(tx.DBTxn)
	}
}

// GetDKGState retrieves the current state of the state machine for the given epoch.
// If an error is returned, the state is undefined meaning that state machine is in initial state
// Error returns:
//   - [storage.ErrNotFound] - if there is no state stored for given epoch, meaning the state machine is in initial state.
func (ds *RecoverablePrivateBeaconKeyStateMachine) GetDKGState(epochCounter uint64) (flow.DKGState, error) {
	var currentState flow.DKGState
	err := ds.db.View(operation.RetrieveDKGStateForEpoch(epochCounter, &currentState))
	return currentState, err
}

// RetrieveMyBeaconPrivateKey retrieves my beacon private key for the given
// epoch, only if my key has been confirmed valid and safe for use.
//
// Returns:
//   - (key, true, nil) if the key is present and confirmed valid
//   - (nil, false, nil) if the key has been marked invalid or unavailable
//     -> no beacon key will ever be available for the epoch in this case
//   - (nil, false, [storage.ErrNotFound]) if the DKG has not ended
//   - (nil, false, error) for any unexpected exception
func (ds *RecoverablePrivateBeaconKeyStateMachine) RetrieveMyBeaconPrivateKey(epochCounter uint64) (key crypto.PrivateKey, safe bool, err error) {
	err = ds.db.View(func(txn *badger.Txn) error {

		// retrieve the end state
		var currentState flow.DKGState
		err = operation.RetrieveDKGStateForEpoch(epochCounter, &currentState)(txn)
		if err != nil {
			key = nil
			safe = false
			return err // storage.ErrNotFound or exception
		}

		// a key is safe iff it was previously committed
		if currentState == flow.RandomBeaconKeyCommitted {
			// retrieve the key - any storage error (including `storage.ErrNotFound`) is an exception
			var encodableKey *encodable.RandomBeaconPrivKey
			encodableKey, err = ds.keyCache.Get(epochCounter)(txn)
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
			return storage.ErrNotFound
		}

		return nil
	})
	return
}

// CommitMyBeaconPrivateKey commits the previously inserted random beacon private key for an epoch.
// Effectively, this method transitions the state machine into the [flow.RandomBeaconKeyCommitted] state if the current state is [flow.DKGStateCompleted].
// Caller needs to supply the [flow.EpochCommit] which is an evidence that the key has been indeed included for the given epoch.
// No errors are expected during normal operations.
func (ds *RecoverablePrivateBeaconKeyStateMachine) CommitMyBeaconPrivateKey(epochCounter uint64, commit *flow.EpochCommit) error {
	return operation.RetryOnConflictTx(ds.db, transaction.Update, func(tx *transaction.Tx) error {
		currentState, err := retrieveCurrentStateTx(epochCounter)(tx.DBTxn)
		if err != nil {
			return err
		}
		// if we are in committed state then there is nothing to do
		if currentState == flow.RandomBeaconKeyCommitted {
			return nil
		}
		key, err := ds.keyCache.Get(epochCounter)(tx.DBTxn)
		if err != nil {
			return storage.NewInvalidDKGStateTransitionErrorf(currentState, flow.RandomBeaconKeyCommitted, "cannot transition without a valid random beacon key: %w", err)
		}

		// verify that the key is part of the EpochCommit
		if err = ensureKeyIncludedInEpoch(epochCounter, key, commit); err != nil {
			return storage.NewInvalidDKGStateTransitionErrorf(currentState, flow.RandomBeaconKeyCommitted,
				"previously storred key has not been found in epoch commit event: %w", err)
		}
		return ds.processStateTransition(epochCounter, currentState, flow.RandomBeaconKeyCommitted)(tx)
	})
}

// UpsertMyBeaconPrivateKey overwrites the random beacon private key for the epoch that recovers the protocol
// from Epoch Fallback Mode. State transitions are allowed if and only if the current state is not equal to
// [flow.RandomBeaconKeyCommitted]. The resulting state of this method call is [flow.RandomBeaconKeyCommitted].
// No errors are expected during normal operations.
func (ds *RecoverablePrivateBeaconKeyStateMachine) UpsertMyBeaconPrivateKey(epochCounter uint64, key crypto.PrivateKey, commit *flow.EpochCommit) error {
	if key == nil {
		return fmt.Errorf("will not store nil beacon key")
	}
	encodableKey := &encodable.RandomBeaconPrivKey{PrivateKey: key}
	err := operation.RetryOnConflictTx(ds.db, transaction.Update, func(tx *transaction.Tx) error {
		currentState, err := retrieveCurrentStateTx(epochCounter)(tx.DBTxn)
		if err != nil {
			return err
		}
		// verify that the key is part of the EpochCommit
		if err = ensureKeyIncludedInEpoch(epochCounter, key, commit); err != nil {
			return storage.NewInvalidDKGStateTransitionErrorf(currentState, flow.RandomBeaconKeyCommitted,
				"previously storred key has not been found in epoch commit event: %w", err)
		}

		// if we are in committed state, we cannot overwrite the key, but we can ignore this input iff the provided key is the same
		if currentState == flow.RandomBeaconKeyCommitted {
			// check if the stored key is equal to the provided key
			storedKey, err := ds.keyCache.Get(epochCounter)(tx.DBTxn)
			if err != nil {
				return irrecoverable.NewExceptionf("could not retrieve a previously committed beacon key for epoch %d: %v", epochCounter, err)
			}
			if key.Equals(storedKey.PrivateKey) {
				return nil
			} else {
				return storage.NewInvalidDKGStateTransitionErrorf(currentState, flow.RandomBeaconKeyCommitted,
					"cannot overwrite previously committed key for epoch: %d", epochCounter)
			}
		}

		err = operation.UpsertMyBeaconPrivateKey(epochCounter, encodableKey)(tx.DBTxn)
		if err != nil {
			return err
		}
		return ds.processStateTransition(epochCounter, currentState, flow.RandomBeaconKeyCommitted)(tx)
	})
	if err != nil {
		return fmt.Errorf("could not overwrite beacon key for epoch %d: %w", epochCounter, err)
	}
	// manually add the key to cache (next line does not touch database)
	ds.keyCache.Insert(epochCounter, encodableKey)
	return nil
}

// ensureKeyIncludedInEpoch performs a sanity check that the key is included in the epoch commit.
// The key is expected to be part of the commit.
// No errors are expected during normal operations.
func ensureKeyIncludedInEpoch(epochCounter uint64, key crypto.PrivateKey, commit *flow.EpochCommit) error {
	if commit.Counter != epochCounter {
		return fmt.Errorf("commit counter does not match epoch counter: %d != %d", commit.Counter, epochCounter)
	}
	publicKey := key.PublicKey()
	if slices.Index(commit.DKGParticipantKeys, publicKey) < 0 {
		return fmt.Errorf("key not included in epoch commit: %s", publicKey)
	}
	return nil
}

// retrieveCurrentStateTx prepares a badger tx which retrieves the current state for the given epoch.
// No errors are expected during normal operations.
func retrieveCurrentStateTx(epochCounter uint64) func(*badger.Txn) (flow.DKGState, error) {
	return func(txn *badger.Txn) (flow.DKGState, error) {
		currentState := flow.DKGStateUninitialized
		err := operation.RetrieveDKGStateForEpoch(epochCounter, &currentState)(txn)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return currentState, fmt.Errorf("could not retrieve current state for epoch %d: %w", epochCounter, err)

		}
		return currentState, nil
	}
}
