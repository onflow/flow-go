package badger_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestDKGState_DKGStarted(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, bstorage.InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store, err := bstorage.NewDKGState(metrics, db)
		require.NoError(t, err)

		epochCounter := rand.Uint64()

		// check dkg-started flag for non-existent epoch
		t.Run("DKGStarted should default to false", func(t *testing.T) {
			started, err := store.GetDKGStarted(rand.Uint64())
			assert.NoError(t, err)
			assert.False(t, started)
		})

		// store dkg-started flag for epoch
		t.Run("should be able to set DKGStarted", func(t *testing.T) {
			err = store.SetDKGStarted(epochCounter)
			assert.NoError(t, err)
		})

		// retrieve flag for epoch
		t.Run("should be able to read DKGStarted", func(t *testing.T) {
			started, err := store.GetDKGStarted(epochCounter)
			assert.NoError(t, err)
			assert.True(t, started)
		})
	})
}

func TestDKGState_BeaconKeys(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, bstorage.InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store, err := bstorage.NewDKGState(metrics, db)
		require.NoError(t, err)

		rand.Seed(time.Now().UnixNano())
		epochCounter := rand.Uint64()

		// attempt to get a non-existent key
		t.Run("should error if retrieving non-existent key", func(t *testing.T) {
			_, err = store.RetrieveMyBeaconPrivateKey(epochCounter)
			assert.True(t, errors.Is(err, storage.ErrNotFound))
		})

		// attempt to store a nil key should fail (use DKGState.SetEndState(flow.DKGEndStateNoKey)
		t.Run("should fail to store a nil key instead)", func(t *testing.T) {
			err = store.InsertMyBeaconPrivateKey(epochCounter, nil)
			assert.Error(t, err)
		})

		// store a key in db
		expected := unittest.RandomBeaconPriv()
		t.Run("should be able to store and read a key", func(t *testing.T) {
			err = store.InsertMyBeaconPrivateKey(epochCounter, expected)
			require.NoError(t, err)
		})

		// retrieve the key by epoch counter
		t.Run("should be able to retrieve stored key", func(t *testing.T) {
			actual, err := store.RetrieveMyBeaconPrivateKey(epochCounter)
			require.NoError(t, err)
			assert.Equal(t, expected, actual)
		})

		// test storing same key
		t.Run("should fail to store a key twice", func(t *testing.T) {
			err = store.InsertMyBeaconPrivateKey(epochCounter, expected)
			require.True(t, errors.Is(err, storage.ErrAlreadyExists))
		})
	})
}

func TestDKGState_EndState(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, bstorage.InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store, err := bstorage.NewDKGState(metrics, db)
		require.NoError(t, err)

		rand.Seed(time.Now().UnixNano())
		epochCounter := rand.Uint64()
		endState := flow.DKGEndStateNoKey

		t.Run("should be able to store an end state", func(t *testing.T) {
			err = store.SetDKGEndState(epochCounter, endState)
			require.NoError(t, err)
		})

		t.Run("should be able to read an end state", func(t *testing.T) {
			readEndState, err := store.GetDKGEndState(epochCounter)
			require.NoError(t, err)
			assert.Equal(t, endState, readEndState)
		})
	})
}

func TestSafeBeaconPrivateKeys(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, bstorage.InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		dkgState, err := bstorage.NewDKGState(metrics, db)
		require.NoError(t, err)
		safeKeys := bstorage.NewSafeBeaconPrivateKeys(dkgState)

		t.Run("non-existent key - should error", func(t *testing.T) {
			epochCounter := rand.Uint64()
			key, safe, err := safeKeys.RetrieveMyBeaconPrivateKey(epochCounter)
			assert.Nil(t, key)
			assert.False(t, safe)
			assert.Error(t, err)
		})

		t.Run("existent key, non-existent dkg end state - should error", func(t *testing.T) {
			epochCounter := rand.Uint64()

			// store a key
			expected := unittest.RandomBeaconPriv().PrivateKey
			err := dkgState.InsertMyBeaconPrivateKey(epochCounter, expected)
			assert.NoError(t, err)

			key, safe, err := safeKeys.RetrieveMyBeaconPrivateKey(epochCounter)
			assert.Nil(t, key)
			assert.False(t, safe)
			assert.Error(t, err)
		})

		t.Run("existent key, unsuccessful dkg - not safe", func(t *testing.T) {
			epochCounter := rand.Uint64()

			// store a key
			expected := unittest.RandomBeaconPriv().PrivateKey
			err := dkgState.InsertMyBeaconPrivateKey(epochCounter, expected)
			assert.NoError(t, err)
			// mark dkg unsuccessful
			err = dkgState.SetDKGEndState(epochCounter, flow.DKGEndStateInconsistentKey)
			assert.NoError(t, err)

			key, safe, err := safeKeys.RetrieveMyBeaconPrivateKey(epochCounter)
			assert.Nil(t, key)
			assert.False(t, safe)
			assert.NoError(t, err)
		})

		t.Run("existent key, successful dkg - safe", func(t *testing.T) {
			epochCounter := rand.Uint64()

			// store a key
			expected := unittest.RandomBeaconPriv().PrivateKey
			err := dkgState.InsertMyBeaconPrivateKey(epochCounter, expected)
			assert.NoError(t, err)
			// mark dkg successful
			err = dkgState.SetDKGEndState(epochCounter, flow.DKGEndStateSuccess)
			assert.NoError(t, err)

			key, safe, err := safeKeys.RetrieveMyBeaconPrivateKey(epochCounter)
			assert.NotNil(t, key)
			assert.True(t, expected.Equals(key))
			assert.True(t, safe)
			assert.NoError(t, err)
		})
	})
}

// TestSecretDBRequirement tests that the DKGState constructor will return an
// error if instantiated using a database not marked with the correct type.
func TestSecretDBRequirement(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		_, err := bstorage.NewDKGState(metrics, db)
		require.Error(t, err)
	})
}
