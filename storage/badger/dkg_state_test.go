package badger

import (
	"math/rand"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestDKGState_UninitializedState verifies that for new epochs, the RecoverableRandomBeaconStateMachine starts
// in the state [flow.DKGStateUninitialized] and reports correct values for that Epoch's DKG state.
// For this test, we start with initial state of the Recoverable Random Beacon State Machine and
// try to perform all possible actions and transitions in it.
func TestDKGState_UninitializedState(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store, err := NewRecoverableRandomBeaconStateMachine(metrics, db)
		require.NoError(t, err)

		setupState := func() uint64 {
			return rand.Uint64()
		}
		epochCounter := setupState()

		started, err := store.IsDKGStarted(epochCounter)
		require.NoError(t, err)
		require.False(t, started)

		actualState, err := store.GetDKGState(epochCounter)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.Equal(t, flow.DKGStateUninitialized, actualState)

		pk, err := store.UnsafeRetrieveMyBeaconPrivateKey(epochCounter)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.Nil(t, pk)

		pk, safe, err := store.RetrieveMyBeaconPrivateKey(epochCounter)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.False(t, safe)
		require.Nil(t, pk)

		t.Run("state transition flow.DKGStateUninitialized -> flow.DKGStateUninitialized should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateUninitialized)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.DKGStateUninitialized ->  flow.DKGStateStarted should be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateStarted)
			require.NoError(t, err)
		})

		t.Run("state transition flow.DKGStateUninitialized -> flow.DKGStateFailure should be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateFailure)
			require.NoError(t, err)
		})

		t.Run("state transition flow.DKGStateUninitialized -> flow.DKGStateCompleted should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateCompleted)
			require.Error(t, err, "should not be able to enter completed state without starting")
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.DKGStateUninitialized -> flow.DKGStateCompleted by inserting a key should not be allowed", func(t *testing.T) {
			err = store.InsertMyBeaconPrivateKey(setupState(), unittest.RandomBeaconPriv())
			require.Error(t, err, "should not be able to enter completed state without starting")
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.DKGStateUninitialized -> flow.RandomBeaconKeyCommitted should be allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.RandomBeaconKeyCommitted)
			require.Error(t, err, "should not be able to set DKG state to recovered, only using dedicated interface")
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
			err = store.UpsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err)
		})
	})
}

// TestDKGState_StartedState verifies that for a DKG in the state [flow.DKGStateStarted], the RecoverableRandomBeaconStateMachine
// reports correct values and permits / rejects state transitions according to the state machine specification.
func TestDKGState_StartedState(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store, err := NewRecoverableRandomBeaconStateMachine(metrics, db)
		require.NoError(t, err)

		setupState := func() uint64 {
			epochCounter := rand.Uint64()
			err = store.SetDKGState(epochCounter, flow.DKGStateStarted)
			require.NoError(t, err)
			return epochCounter
		}
		epochCounter := setupState()

		actualState, err := store.GetDKGState(epochCounter)
		require.NoError(t, err)
		require.Equal(t, flow.DKGStateStarted, actualState)

		started, err := store.IsDKGStarted(epochCounter)
		require.NoError(t, err)
		require.True(t, started)

		pk, err := store.UnsafeRetrieveMyBeaconPrivateKey(epochCounter)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.Nil(t, pk)

		pk, safe, err := store.RetrieveMyBeaconPrivateKey(epochCounter)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.False(t, safe)
		require.Nil(t, pk)

		t.Run("state transition flow.DKGStateStarted -> flow.DKGStateUninitialized should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateUninitialized)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.DKGStateStarted -> flow.DKGStateStarted should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateStarted)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.DKGStateStarted -> flow.DKGStateFailure should be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateFailure)
			require.NoError(t, err)
		})

		t.Run("state transition flow.DKGStateStarted -> flow.DKGStateCompleted should be rejected if no key was inserted first", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateCompleted)
			require.Error(t, err, "should not be able to enter completed state without providing a private key")
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.DKGStateStarted -> flow.DKGStateCompleted should be allowed, but only via inserting a key", func(t *testing.T) {
			epochCounter := setupState()
			err = store.InsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err)
			resultingState, err := store.GetDKGState(epochCounter)
			require.NoError(t, err)
			require.Equal(t, flow.DKGStateCompleted, resultingState)
		})

		t.Run("while state transition flow.DKGStateStarted -> flow.RandomBeaconKeyCommitted is allowed, it should not proceed without a key being inserted first", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.RandomBeaconKeyCommitted)
			require.Error(t, err, "should not be able to set DKG state to recovered, only using dedicated interface")
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.DKGStateStarted -> flow.RandomBeaconKeyCommitted should be allowed, but only via upserting a key", func(t *testing.T) {
			epochCounter := setupState()
			err = store.UpsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err)
			resultingState, err := store.GetDKGState(epochCounter)
			require.NoError(t, err)
			require.Equal(t, flow.RandomBeaconKeyCommitted, resultingState)
		})

	})
}

// TestDKGState_CompletedState  verifies that for a DKG in the state [flow.DKGStateCompleted], the RecoverableRandomBeaconStateMachine
// reports correct values and permits / rejects state transitions according to the state machine specification.
func TestDKGState_CompletedState(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store, err := NewRecoverableRandomBeaconStateMachine(metrics, db)
		require.NoError(t, err)

		setupState := func() uint64 {
			epochCounter := rand.Uint64()
			err = store.SetDKGState(epochCounter, flow.DKGStateStarted)
			require.NoError(t, err)
			err = store.InsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err)
			return epochCounter
		}
		epochCounter := setupState()

		actualState, err := store.GetDKGState(epochCounter)
		require.NoError(t, err)
		require.Equal(t, flow.DKGStateCompleted, actualState)

		started, err := store.IsDKGStarted(epochCounter)
		require.NoError(t, err)
		require.True(t, started)

		pk, err := store.UnsafeRetrieveMyBeaconPrivateKey(epochCounter)
		require.NoError(t, err)
		require.NotNil(t, pk)

		pk, safe, err := store.RetrieveMyBeaconPrivateKey(epochCounter)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.False(t, safe)
		require.Nil(t, pk)

		t.Run("state transition flow.DKGStateCompleted -> flow.DKGStateUninitialized should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateUninitialized)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.DKGStateCompleted -> flow.DKGStateStarted should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateStarted)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.DKGStateCompleted -> flow.DKGStateFailure should be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateFailure)
			require.NoError(t, err)
		})

		t.Run("state transition flow.DKGStateCompleted -> flow.DKGStateCompleted should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateCompleted)
			require.Error(t, err, "already in this state")
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))

			err = store.InsertMyBeaconPrivateKey(setupState(), unittest.RandomBeaconPriv())
			require.Error(t, err, "already inserted private key")
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})

		t.Run("state transition flow.DKGStateCompleted -> flow.RandomBeaconKeyCommitted should be allowed, because key is already stored", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.RandomBeaconKeyCommitted)
			require.NoError(t, err, "should be allowed since we have a stored private key")
			resultingState, err := store.GetDKGState(epochCounter)
			require.NoError(t, err)
			require.Equal(t, flow.RandomBeaconKeyCommitted, resultingState)
		})

		t.Run("state transition flow.DKGStateCompleted -> flow.RandomBeaconKeyCommitted (recovery, overwriting existing key) should be allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.UpsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err)
			resultingState, err := store.GetDKGState(epochCounter)
			require.NoError(t, err)
			require.Equal(t, flow.RandomBeaconKeyCommitted, resultingState)
		})
	})
}

// TestDKGState_StartedState verifies that for a DKG in the state [flow.DKGStateFailure], the RecoverableRandomBeaconStateMachine
// reports correct values and permits / rejects state transitions according to the state machine specification.
func TestDKGState_FailureState(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store, err := NewRecoverableRandomBeaconStateMachine(metrics, db)
		require.NoError(t, err)

		setupState := func() uint64 {
			epochCounter := rand.Uint64()
			err = store.SetDKGState(epochCounter, flow.DKGStateFailure)
			require.NoError(t, err)
			return epochCounter
		}
		epochCounter := setupState()

		actualState, err := store.GetDKGState(epochCounter)
		require.NoError(t, err)
		require.Equal(t, flow.DKGStateFailure, actualState)

		started, err := store.IsDKGStarted(epochCounter)
		require.NoError(t, err)
		require.True(t, started)

		pk, err := store.UnsafeRetrieveMyBeaconPrivateKey(epochCounter)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.Nil(t, pk)

		pk, safe, err := store.RetrieveMyBeaconPrivateKey(epochCounter)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.False(t, safe)
		require.Nil(t, pk)

		t.Run("state transition flow.DKGStateFailure -> flow.DKGStateUninitialized should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateUninitialized)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.DKGStateFailure -> flow.DKGStateStarted should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateStarted)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.DKGStateFailure -> flow.DKGStateFailure should be allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.DKGStateFailure)
			require.NoError(t, err)
			resultingState, err := store.GetDKGState(epochCounter)
			require.NoError(t, err)
			require.Equal(t, flow.DKGStateFailure, resultingState)
		})

		t.Run("state transition flow.DKGStateFailure -> flow.DKGStateCompleted should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateCompleted)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.DKGStateFailure -> flow.DKGStateCompleted by inserting a key should not be allowed", func(t *testing.T) {
			err = store.InsertMyBeaconPrivateKey(setupState(), unittest.RandomBeaconPriv())
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.DKGStateFailure -> flow.RandomBeaconKeyCommitted is allowed, it should not proceed without a key being inserted first", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.RandomBeaconKeyCommitted)
			require.Error(t, err, "should not be able to set DKG state to recovered, only using dedicated interface")
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})
		t.Run("state transition flow.DKGStateFailure -> flow.RandomBeaconKeyCommitted should be allowed via upserting the key (recovery path)", func(t *testing.T) {
			epochCounter := setupState()
			expectedKey := unittest.RandomBeaconPriv()
			err = store.UpsertMyBeaconPrivateKey(epochCounter, expectedKey)
			require.NoError(t, err)
			actualKey, safe, err := store.RetrieveMyBeaconPrivateKey(epochCounter)
			require.NoError(t, err)
			require.True(t, safe)
			require.Equal(t, expectedKey, actualKey)
			resultingState, err := store.GetDKGState(epochCounter)
			require.NoError(t, err)
			require.Equal(t, flow.RandomBeaconKeyCommitted, resultingState)
		})
	})
}

// TestDKGState_RandomBeaconKeyCommittedState verifies that for a DKG in the state [flow.RandomBeaconKeyCommitted], the RecoverableRandomBeaconStateMachine
// reports correct values and permits / rejects state transitions according to the state machine specification.
func TestDKGState_RandomBeaconKeyCommittedState(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store, err := NewRecoverableRandomBeaconStateMachine(metrics, db)
		require.NoError(t, err)

		setupState := func() uint64 {
			epochCounter := rand.Uint64()
			err = store.UpsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err)
			return epochCounter
		}
		epochCounter := setupState()

		actualState, err := store.GetDKGState(epochCounter)
		require.NoError(t, err)
		require.Equal(t, flow.RandomBeaconKeyCommitted, actualState)

		started, err := store.IsDKGStarted(epochCounter)
		require.NoError(t, err)
		require.True(t, started)

		pk, err := store.UnsafeRetrieveMyBeaconPrivateKey(epochCounter)
		require.NoError(t, err)
		require.NotNil(t, pk)

		pk, safe, err := store.RetrieveMyBeaconPrivateKey(epochCounter)
		require.NoError(t, err)
		require.True(t, safe)
		require.NotNil(t, pk)

		t.Run("state transition flow.RandomBeaconKeyCommitted -> flow.DKGStateUninitialized should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateUninitialized)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.RandomBeaconKeyCommitted -> flow.DKGStateStarted should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateStarted)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.RandomBeaconKeyCommitted -> flow.DKGStateFailure should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateFailure)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.RandomBeaconKeyCommitted -> flow.DKGStateCompleted should not be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateCompleted)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("state transition flow.RandomBeaconKeyCommitted -> flow.DKGStateCompleted by inserting a key should not be allowed", func(t *testing.T) {
			err = store.InsertMyBeaconPrivateKey(setupState(), unittest.RandomBeaconPriv())
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})

		t.Run("state transition flow.RandomBeaconKeyCommitted -> flow.RandomBeaconKeyCommitted should be idempotent for same key", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.RandomBeaconKeyCommitted)
			require.NoError(t, err, "should be possible as we are not changing the private key")
			resultingState, err := store.GetDKGState(epochCounter)
			require.NoError(t, err)
			require.Equal(t, flow.RandomBeaconKeyCommitted, resultingState)

			err = store.UpsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err, "should be possible ONLY for the same private key")
			resultingState, err = store.GetDKGState(epochCounter)
			require.NoError(t, err)
			require.Equal(t, flow.RandomBeaconKeyCommitted, resultingState)
		})
	})
}

// TestSecretDBRequirement tests that the RecoverablePrivateBeaconKeyStateMachine constructor will return an
// error if instantiated using a database not marked with the correct type.
func TestSecretDBRequirement(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		_, err := NewRecoverableRandomBeaconStateMachine(metrics, db)
		require.Error(t, err)
	})
}
