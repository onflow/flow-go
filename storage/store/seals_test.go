package store_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRetrieveWithoutStore(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		s := store.NewSeals(metrics, db)

		_, err := s.ByID(unittest.IdentifierFixture())
		require.ErrorIs(t, err, storage.ErrNotFound)

		_, err = s.HighestInFork(unittest.IdentifierFixture())
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

// TestSealStoreRetrieve verifies that a seal can be stored and retrieved by its ID
func TestSealStoreRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		s := store.NewSeals(metrics, db)

		expected := unittest.Seal.Fixture()
		// store seal
		err := s.Store(expected)
		require.NoError(t, err)

		// retrieve seal
		seal, err := s.ByID(expected.ID())
		require.NoError(t, err)
		require.Equal(t, expected, seal)
	})
}

// TestSealIndexAndRetrieve verifies that:
//   - for a block, we can s (aka index) the latest sealed block along this fork.
//
// Note: indexing the seal for a block is currently implemented only through a direct
// Badger operation. The Seals mempool only supports retrieving the latest sealed block.
func TestSealIndexAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		s := store.NewSeals(metrics, db)

		expectedSeal := unittest.Seal.Fixture()
		blockID := unittest.IdentifierFixture()

		// store the seal first
		err := s.Store(expectedSeal)
		require.NoError(t, err)

		// index the seal ID for the heighest sealed block in this fork
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexLatestSealAtBlock(lctx, rw.Writer(), blockID, expectedSeal.ID())
			})
		})
		require.NoError(t, err)

		// retrieve latest seal
		seal, err := s.HighestInFork(blockID)
		require.NoError(t, err)
		require.Equal(t, expectedSeal, seal)
	})
}

// TestSealedBlockIndexAndRetrieve checks after indexing a seal by a sealed block ID, it can be
// retrieved by the sealed block ID
func TestSealedBlockIndexAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		s := store.NewSeals(metrics, db)

		expectedSeal := unittest.Seal.Fixture()
		blockID := unittest.IdentifierFixture()
		expectedSeal.BlockID = blockID

		// store the seal first
		err := s.Store(expectedSeal)
		require.NoError(t, err)

		// index the seal ID for the highest sealed block in this fork
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexFinalizedSealByBlockID(rw.Writer(), expectedSeal.BlockID, expectedSeal.ID())
		})
		require.NoError(t, err)

		// retrieve latest seal
		seal, err := s.FinalizedSealForBlock(blockID)
		require.NoError(t, err)
		require.Equal(t, expectedSeal, seal)
	})
}
