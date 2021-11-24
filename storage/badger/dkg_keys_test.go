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
		_, err = store.RetrieveMyBeaconPrivateKey(epochCounter)
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store a key in db
		expected := unittest.RandomBeaconPriv()
		err = store.InsertMyBeaconPrivateKey(epochCounter, expected)
		require.NoError(t, err)

		// retrieve the key by epoch counter
		actual, err := store.RetrieveMyBeaconPrivateKey(epochCounter)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// test storing same key
		err = store.InsertMyBeaconPrivateKey(epochCounter, expected)
		require.True(t, errors.Is(err, storage.ErrAlreadyExists))
	})
}
