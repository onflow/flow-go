package badger

import (
	"math/rand"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestDKGState_UninitializedState checks that invariants are enforced for uninitialized DKG state.
// This test is written in a way that we start with initial state of the Recoverable Random Beacon State Machine and
// try to perform all possible actions and transitions in it.
func TestDKGState_UninitializedState(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store, err := NewDKGState(metrics, db)
		require.NoError(t, err)

		epochCounter := rand.Uint64()

		started, err := store.GetDKGStarted(epochCounter)
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

		t.Run("-> flow.DKGStateUninitialized, not allowed", func(t *testing.T) {
			epochCounter++
			err = store.SetDKGState(epochCounter, flow.DKGStateUninitialized)
			require.Error(t, err)
		})

		t.Run("-> flow.DKGStateStarted, should be allowed", func(t *testing.T) {
			epochCounter++
			err = store.SetDKGState(epochCounter, flow.DKGStateStarted)
			require.NoError(t, err)
		})

		t.Run("-> flow.DKGStateFailure, should be allowed", func(t *testing.T) {
			epochCounter++
			err = store.SetDKGState(epochCounter, flow.DKGStateFailure)
			require.NoError(t, err)
		})

		t.Run("-> flow.DKGStateCompleted, not allowed", func(t *testing.T) {
			epochCounter++
			err = store.InsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.Error(t, err, "should not be able to enter completed state without starting")
			err = store.SetDKGState(epochCounter, flow.DKGStateCompleted)
			require.Error(t, err, "should not be able to enter completed state without starting")
		})

		t.Run("-> flow.RandomBeaconKeyCommitted, should be allowed", func(t *testing.T) {
			epochCounter++
			err = store.SetDKGState(epochCounter, flow.RandomBeaconKeyCommitted)
			require.Error(t, err, "should not be able to set DKG state to recovered, only using dedicated interface")
			err = store.UpsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err)
		})
	})
}

// TestDKGState_UninitializedState checks that invariants are enforced for uninitialized DKG state.
// This test is written in a way that we start with initial state of the Recoverable Random Beacon State Machine and
// try to perform all possible actions and transitions in it.
func TestDKGState_StartedState(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store, err := NewDKGState(metrics, db)
		require.NoError(t, err)

		setupState := func() uint64 {
			epochCounter := rand.Uint64()
			err = store.SetDKGState(epochCounter, flow.DKGStateStarted)
			require.NoError(t, err)
			return epochCounter
		}
		epochCounter := setupState()

		actualState, err := store.GetDKGState(epochCounter)
		require.NoError(t, err, storage.ErrNotFound)
		require.Equal(t, flow.DKGStateStarted, actualState)

		started, err := store.GetDKGStarted(epochCounter)
		require.NoError(t, err)
		require.True(t, started)

		pk, err := store.UnsafeRetrieveMyBeaconPrivateKey(epochCounter)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.Nil(t, pk)

		pk, safe, err := store.RetrieveMyBeaconPrivateKey(epochCounter)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.False(t, safe)
		require.Nil(t, pk)

		t.Run("-> flow.DKGStateUninitialized, not allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateUninitialized)
			require.Error(t, err)
		})

		t.Run("-> flow.DKGStateStarted, not allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateStarted)
			require.Error(t, err)
		})

		t.Run("-> flow.DKGStateFailure, should be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateFailure)
			require.NoError(t, err)
		})

		t.Run("-> flow.DKGStateCompleted, should be allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.DKGStateCompleted)
			require.Error(t, err, "should not be able to enter completed state without providing a private key")
			err = store.InsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err)
		})

		t.Run("-> flow.RandomBeaconKeyCommitted, should be allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.RandomBeaconKeyCommitted)
			require.Error(t, err, "should not be able to set DKG state to recovered, only using dedicated interface")
			err = store.UpsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.NoError(t, err)
		})
	})
}

// TestDKGState_UninitializedState checks that invariants are enforced for uninitialized DKG state.
// This test is written in a way that we start with initial state of the Recoverable Random Beacon State Machine and
// try to perform all possible actions and transitions in it.
func TestDKGState_CompletedState(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store, err := NewDKGState(metrics, db)
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
		require.NoError(t, err, storage.ErrNotFound)
		require.Equal(t, flow.DKGStateCompleted, actualState)

		started, err := store.GetDKGStarted(epochCounter)
		require.NoError(t, err)
		require.True(t, started)

		pk, err := store.UnsafeRetrieveMyBeaconPrivateKey(epochCounter)
		require.NoError(t, err)
		require.NotNil(t, pk)

		pk, safe, err := store.RetrieveMyBeaconPrivateKey(epochCounter)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.False(t, safe)
		require.Nil(t, pk)

		t.Run("-> flow.DKGStateUninitialized, not allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateUninitialized)
			require.Error(t, err)
		})

		t.Run("-> flow.DKGStateStarted, not allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateStarted)
			require.Error(t, err)
		})

		t.Run("-> flow.DKGStateFailure, should be allowed", func(t *testing.T) {
			err = store.SetDKGState(setupState(), flow.DKGStateFailure)
			require.NoError(t, err)
		})

		t.Run("-> flow.DKGStateCompleted, not allowed", func(t *testing.T) {
			epochCounter := setupState()
			err = store.SetDKGState(epochCounter, flow.DKGStateCompleted)
			require.Error(t, err, "already in this state")
			err = store.InsertMyBeaconPrivateKey(epochCounter, unittest.RandomBeaconPriv())
			require.Error(t, err, "already inserted private key")
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

func TestSafeBeaconPrivateKeys(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		dkgState, err := NewDKGState(metrics, db)
		require.NoError(t, err)

		t.Run("non-existent key -> should return ErrNotFound", func(t *testing.T) {
			epochCounter := rand.Uint64()
			key, safe, err := dkgState.RetrieveMyBeaconPrivateKey(epochCounter)
			assert.Nil(t, key)
			assert.False(t, safe)
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("existent key, non-existent end state -> should return ErrNotFound", func(t *testing.T) {
			epochCounter := rand.Uint64()

			// store a key
			expected := unittest.RandomBeaconPriv().PrivateKey
			err := dkgState.InsertMyBeaconPrivateKey(epochCounter, expected)
			assert.NoError(t, err)

			key, safe, err := dkgState.RetrieveMyBeaconPrivateKey(epochCounter)
			assert.Nil(t, key)
			assert.False(t, safe)
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("existent key, unsuccessful end state -> not safe", func(t *testing.T) {
			epochCounter := rand.Uint64()

			// store a key
			expected := unittest.RandomBeaconPriv().PrivateKey
			err := dkgState.InsertMyBeaconPrivateKey(epochCounter, expected)
			assert.NoError(t, err)
			// mark dkg unsuccessful
			err = dkgState.SetDKGState(epochCounter, flow.DKGStateFailure)
			assert.NoError(t, err)

			key, safe, err := dkgState.RetrieveMyBeaconPrivateKey(epochCounter)
			assert.Nil(t, key)
			assert.False(t, safe)
			assert.NoError(t, err)
		})

		t.Run("existent key, inconsistent key end state -> not safe", func(t *testing.T) {
			epochCounter := rand.Uint64()

			// store a key
			expected := unittest.RandomBeaconPriv().PrivateKey
			err := dkgState.InsertMyBeaconPrivateKey(epochCounter, expected)
			assert.NoError(t, err)
			// mark dkg result as inconsistent
			err = dkgState.SetDKGState(epochCounter, flow.DKGStateFailure)
			assert.NoError(t, err)

			key, safe, err := dkgState.RetrieveMyBeaconPrivateKey(epochCounter)
			assert.Nil(t, key)
			assert.False(t, safe)
			assert.NoError(t, err)
		})

		t.Run("non-existent key, no key end state -> not safe", func(t *testing.T) {
			epochCounter := rand.Uint64()

			// mark dkg result as no key
			err = dkgState.SetDKGState(epochCounter, flow.DKGStateFailure)
			assert.NoError(t, err)

			key, safe, err := dkgState.RetrieveMyBeaconPrivateKey(epochCounter)
			assert.Nil(t, key)
			assert.False(t, safe)
			assert.NoError(t, err)
		})

		t.Run("existent key, successful end state -> safe", func(t *testing.T) {
			epochCounter := rand.Uint64()

			// store a key
			expected := unittest.RandomBeaconPriv().PrivateKey
			err := dkgState.InsertMyBeaconPrivateKey(epochCounter, expected)
			assert.NoError(t, err)
			// mark dkg successful
			err = dkgState.SetDKGState(epochCounter, flow.RandomBeaconKeyCommitted)
			assert.NoError(t, err)

			key, safe, err := dkgState.RetrieveMyBeaconPrivateKey(epochCounter)
			assert.NotNil(t, key)
			assert.True(t, expected.Equals(key))
			assert.True(t, safe)
			assert.NoError(t, err)
		})

		t.Run("non-existent key, successful end state -> exception!", func(t *testing.T) {
			epochCounter := rand.Uint64()

			// mark dkg successful
			err = dkgState.SetDKGState(epochCounter, flow.RandomBeaconKeyCommitted)
			assert.NoError(t, err)

			key, safe, err := dkgState.RetrieveMyBeaconPrivateKey(epochCounter)
			assert.Nil(t, key)
			assert.False(t, safe)
			assert.Error(t, err)
			assert.NotErrorIs(t, err, storage.ErrNotFound)
		})

	})
}

// TestSecretDBRequirement tests that the RecoverablePrivateBeaconKeyState constructor will return an
// error if instantiated using a database not marked with the correct type.
func TestSecretDBRequirement(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		_, err := NewDKGState(metrics, db)
		require.Error(t, err)
	})
}
