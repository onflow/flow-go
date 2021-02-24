package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// TestEpochSetupStoreAndRetrieve tests that a setup can be stored, retrieved and attempted to be stored again without an error
func TestEpochSetupStoreAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewEpochSetups(metrics, db)

		// attempt to get a setup that doesn't exist
		_, err := store.ByID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store a setup in db
		expected := unittest.EpochSetupFixture()
		err = operation.RetryOnConflict(db.Update, store.StoreTx(expected))
		require.NoError(t, err)

		// retrieve the setup by ID
		actual, err := store.ByID(expected.ID())
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// test storing same epoch setup
		err = operation.RetryOnConflict(db.Update, store.StoreTx(expected))
		require.True(t, errors.Is(err, storage.ErrAlreadyExists))

		// test get by view
		middleView := expected.FirstView + (expected.FinalView-expected.FirstView)/2
		counter, err := store.CounterByView(middleView)
		require.NoError(t, err)
		require.Equal(t, expected.Counter, counter)
	})
}
