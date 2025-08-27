package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEpochCommitStoreAndRetrieve tests that a commit can be stored, retrieved and attempted to be stored again without an error
func TestEpochCommitStoreAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewEpochCommits(metrics, db)

		// attempt to get a invalid commit
		_, err := store.ByID(unittest.IdentifierFixture())
		assert.ErrorIs(t, err, storage.ErrNotFound)

		// store a commit in db
		expected := unittest.EpochCommitFixture()
		err = transaction.Update(db, func(tx *transaction.Tx) error {
			return store.StoreTx(expected)(tx)
		})
		require.NoError(t, err)

		// retrieve the commit by ID
		actual, err := store.ByID(expected.ID())
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// test storing same epoch commit
		err = transaction.Update(db, func(tx *transaction.Tx) error {
			return store.StoreTx(expected)(tx)
		})
		require.NoError(t, err)
	})
}
