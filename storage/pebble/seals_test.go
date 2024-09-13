package pebble_test

import (
	"errors"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/utils/unittest"

	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

func TestRetrieveWithoutStore(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		store := pebblestorage.NewSeals(metrics, db)

		_, err := store.ByID(unittest.IdentifierFixture())
		require.True(t, errors.Is(err, storage.ErrNotFound))

		_, err = store.HighestInFork(unittest.IdentifierFixture())
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

// TestSealStoreRetrieve verifies that a seal can be stored and retrieved by its ID
func TestSealStoreRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		store := pebblestorage.NewSeals(metrics, db)

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
//   - for a block, we can store (aka index) the latest sealed block along this fork.
//
// Note: indexing the seal for a block is currently implemented only through a direct
// pebble operation. The Seals mempool only supports retrieving the latest sealed block.
func TestSealIndexAndRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		store := pebblestorage.NewSeals(metrics, db)

		expectedSeal := unittest.Seal.Fixture()
		blockID := unittest.IdentifierFixture()

		// store the seal first
		err := store.Store(expectedSeal)
		require.NoError(t, err)

		// index the seal ID for the heighest sealed block in this fork
		err = operation.IndexLatestSealAtBlock(blockID, expectedSeal.ID())(db)
		require.NoError(t, err)

		// retrieve latest seal
		seal, err := store.HighestInFork(blockID)
		require.NoError(t, err)
		require.Equal(t, expectedSeal, seal)
	})
}

// TestSealedBlockIndexAndRetrieve checks after indexing a seal by a sealed block ID, it can be
// retrieved by the sealed block ID
func TestSealedBlockIndexAndRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		store := pebblestorage.NewSeals(metrics, db)

		expectedSeal := unittest.Seal.Fixture()
		blockID := unittest.IdentifierFixture()
		expectedSeal.BlockID = blockID

		// store the seal first
		err := store.Store(expectedSeal)
		require.NoError(t, err)

		// index the seal ID for the highest sealed block in this fork
		err = operation.IndexFinalizedSealByBlockID(expectedSeal.BlockID, expectedSeal.ID())(db)
		require.NoError(t, err)

		// retrieve latest seal
		seal, err := store.FinalizedSealForBlock(blockID)
		require.NoError(t, err)
		require.Equal(t, expectedSeal, seal)
	})
}
