package pebble_test

import (
	"errors"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

// TestEpochSetupStoreAndRetrieve tests that a setup can be stored, retrieved and attempted to be stored again without an error
func TestEpochSetupStoreAndRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		store := pebblestorage.NewEpochSetups(metrics, db)

		// attempt to get a setup that doesn't exist
		_, err := store.ByID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store a setup in db
		expected := unittest.EpochSetupFixture()
		err = store.StoreTx(expected)(db)
		require.NoError(t, err)

		// retrieve the setup by ID
		actual, err := store.ByID(expected.ID())
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// test storing same epoch setup
		err = store.StoreTx(expected)(db)
		require.NoError(t, err)
	})
}
