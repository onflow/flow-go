package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
)

func TestRetrieveWithoutStore(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewSeals(metrics, db)

		_, err := store.ByID(unittest.IdentifierFixture())
		require.True(t, errors.Is(err, storage.ErrNotFound))

		_, err = store.ByBlockID(unittest.IdentifierFixture())
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

// TestSealStoreRetrieve verifies that a seal can be stored and retrieved by its ID
func TestSealStoreRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewSeals(metrics, db)

		expected := unittest.Seal.Fixture()
		// store seal
		err := store.Store(expected)
		require.NoError(t, err)

		// retrieve seal
		seal, err := store.ByID(expected.ID())
		require.NoError(t, err)
		require.Equal(t, expected, seal)
	})
}

// TestSealIndexAndRetrieve verifies that:
//  * for a block, we can store (aka index) the latest sealed block along this fork.
// Note: indexing the seal for a block is currently implemented only through a direct
// Badger operation. The Seals mempool only supports retrieving the latest sealed block.
func TestSealIndexAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewSeals(metrics, db)

		expectedSeal := unittest.Seal.Fixture()
		blockID := unittest.IdentifierFixture()

		// store the seal first
		err := store.Store(expectedSeal)
		require.NoError(t, err)

		// index the seal ID for the heighest sealed block in this fork
		err = operation.RetryOnConflict(db.Update, operation.IndexBlockSeal(blockID, expectedSeal.ID()))
		require.NoError(t, err)

		// retrieve latest seal
		seal, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, expectedSeal, seal)
	})
}
