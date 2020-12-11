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

// TestEpochSetupStoreAndRetrieve tests that a commit can be stored, retrieved and attempted to be stored again without an error
func TestEpochSetupStoreAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewEpochSetups(metrics, db)

		// attempt to get a invalid commit
		_, err := store.ByID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store a commit in db
		expected := unittest.EpochSetupFixture()
		err = operation.RetryOnConflict(db.Update, store.StoreTx(expected))
		require.NoError(t, err)

		// retrieve the commit by ID
		actual, err := store.ByID(expected.ID())
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// test storing same epoch setup
		err = operation.RetryOnConflict(db.Update, store.StoreTx(expected))
		require.True(t, errors.Is(err, storage.ErrAlreadyExists))
	})
}
