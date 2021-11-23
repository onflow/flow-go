package badger_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestSecretDBRequirement tests that the DKGKeys constructor will return an
// error if instantiated using a database not marked with the correct type.
func TestSecretDBRequirement(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		_, err := bstorage.NewDKGKeys(metrics, db)
		require.Error(t, err)
	})
}

func TestDKGKeysInsertAndRetrieve(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, bstorage.InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store, err := bstorage.NewDKGKeys(metrics, db)
		require.NoError(t, err)

		rand.Seed(time.Now().UnixNano())
		epochCounter := rand.Uint64()

		// attempt to get a non-existent key
		_, _, err = store.RetrieveMyDKGPrivateInfo(epochCounter)
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store a key in db
		expected := unittest.DKGParticipantPriv()
		err = store.InsertMyDKGPrivateInfo(epochCounter, expected)
		require.NoError(t, err)

		// retrieve the key by epoch counter
		actual, hasDKGKey, err := store.RetrieveMyDKGPrivateInfo(epochCounter)
		require.NoError(t, err)
		require.True(t, hasDKGKey)
		assert.Equal(t, expected, actual)

		// test storing same key
		err = store.InsertMyDKGPrivateInfo(epochCounter, expected)
		require.True(t, errors.Is(err, storage.ErrAlreadyExists))
	})
}

func TestDKGKeysInsertAndRetrieveNoDKGKey(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, bstorage.InitSecret, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store, err := bstorage.NewDKGKeys(metrics, db)
		require.NoError(t, err)

		rand.Seed(time.Now().UnixNano())
		epochCounter := rand.Uint64()

		// store a key in db
		err = store.InsertNoDKGPrivateInfo(epochCounter)
		require.NoError(t, err)

		// retrieve the key by epoch counter
		_, hasDKGKey, err := store.RetrieveMyDKGPrivateInfo(epochCounter)
		require.NoError(t, err)
		require.False(t, hasDKGKey)
	})
}
