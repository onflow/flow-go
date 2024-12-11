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
			epochCounter := setupState()
			err = store.InsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.Error(t, err, "should not be able to enter completed state without starting")
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
			err = store.SetDKGState(epochCounter, flow.DKGStateCompleted)
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

		t.Run("-> flow.DKGStateCompleted, should be allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.DKGStateCompleted)
			require.Error(t, err, "should not be able to enter completed state without providing a private key")
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
			err = store.InsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err)
		})

		t.Run("state transition flow.DKGStateStarted -> flow.RandomBeaconKeyCommitted should be allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.RandomBeaconKeyCommitted)
			require.Error(t, err, "should not be able to set DKG state to recovered, only using dedicated interface")
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
			err = store.UpsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err)
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

		t.Run("-> flow.DKGStateCompleted, not allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.DKGStateCompleted)
			require.Error(t, err, "already in this state")
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
			err = store.InsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.Error(t, err, "already inserted private key")
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})

		t.Run("-> flow.RandomBeaconKeyCommitted, should be allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.RandomBeaconKeyCommitted)
			require.NoError(t, err, "should be allowed since we have a stored private key")
		})

		t.Run("-> flow.RandomBeaconKeyCommitted(recovery), should be allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.UpsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err)
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

		t.Run("-> flow.DKGStateFailure, should be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateFailure)
			require.NoError(t, err)
		})

		t.Run("-> flow.DKGStateCompleted, not allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.DKGStateCompleted)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
			err = store.InsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
		})

		t.Run("-> flow.RandomBeaconKeyCommitted, should be allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.RandomBeaconKeyCommitted)
			require.Error(t, err, "should not be able to set state to RandomBeaconKeyCommitted, without a key being inserted first")
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
			expectedKey := unittest.RandomBeaconPriv()
			err = store.UpsertMyBeaconPrivateKey(epochCounter, expectedKey)
			require.NoError(t, err)
			actualKey, safe, err := store.RetrieveMyBeaconPrivateKey(epochCounter)
			require.NoError(t, err)
			require.True(t, safe)
			require.Equal(t, expectedKey, actualKey)
		})
	})
}

// TestDKGState_StartedState verifies that for a DKG in the state [flow.RandomBeaconKeyCommitted], the RecoverableRandomBeaconStateMachine
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

		t.Run("-> flow.DKGStateCompleted, not allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.DKGStateCompleted)
			require.Error(t, err)
			require.True(t, storage.IsInvalidDKGStateTransitionError(err))
			err = store.InsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})

		t.Run("-> flow.RandomBeaconKeyCommitted, allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.RandomBeaconKeyCommitted)
			require.NoError(t, err, "should be possible since we have a stored private key")
			err = store.UpsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err)
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
