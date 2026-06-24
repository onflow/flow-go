package store_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBatchStoringLightTransactionResults(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		txResultsStore := store.NewLightTransactionResults(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txResults := getLightTransactionResultsFixture(10)

		t.Run("batch txResultsStore results", func(t *testing.T) {
			require.NoError(t, unittest.WithLock(t, lockManager, storage.LockInsertLightTransactionResult, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return txResultsStore.BatchStore(lctx, rw, blockID, txResults)
				})
			}))

			// add a results to a new block to validate they are not included in lookups
			require.NoError(t, unittest.WithLock(t, lockManager, storage.LockInsertLightTransactionResult, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return txResultsStore.BatchStore(lctx, rw, unittest.IdentifierFixture(), getLightTransactionResultsFixture(2))
				})
			}))

		})

		t.Run("read results with cache", func(t *testing.T) {
			for _, txResult := range txResults {
				actual, err := txResultsStore.ByBlockIDTransactionID(blockID, txResult.TransactionID)
				require.NoError(t, err)
				assert.Equal(t, txResult, *actual)
			}
		})

		newStore := store.NewLightTransactionResults(metrics, db, 1000)
		t.Run("read results without cache", func(t *testing.T) {
			// test loading from database (without cache)
			// create a new instance using the same db so it has an empty cache
			for _, txResult := range txResults {
				actual, err := newStore.ByBlockIDTransactionID(blockID, txResult.TransactionID)
				require.NoError(t, err)
				assert.Equal(t, txResult, *actual)
			}
		})

		t.Run("cached and non-cached results are equal", func(t *testing.T) {
			// check retrieving by index from both cache and db
			for i := len(txResults) - 1; i >= 0; i-- {
				actual, err := txResultsStore.ByBlockIDTransactionIndex(blockID, uint32(i))
				require.NoError(t, err)
				assert.Equal(t, txResults[i], *actual)

				actual, err = newStore.ByBlockIDTransactionIndex(blockID, uint32(i))
				require.NoError(t, err)
				assert.Equal(t, txResults[i], *actual)
			}
		})

		t.Run("read all results for block", func(t *testing.T) {
			actuals, err := txResultsStore.ByBlockID(blockID)
			require.NoError(t, err)

			assert.Equal(t, len(txResults), len(actuals))
			for i := range txResults {
				assert.Equal(t, txResults[i], actuals[i])
			}
		})
	})
}

func TestReadingNotStoredLightTransactionResults(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		txResultsStore := store.NewLightTransactionResults(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()
		txIndex := rand.Uint32()

		_, err := txResultsStore.ByBlockIDTransactionID(blockID, txID)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		_, err = txResultsStore.ByBlockIDTransactionIndex(blockID, txIndex)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}

// Test that attempting to batch store light transaction results for a block ID that already exists
// results in a [storage.ErrAlreadyExists] error, and that the original data remains unchanged.
func TestBatchStoreLightTransactionResultsErrAlreadyExists(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		txResultsStore := store.NewLightTransactionResults(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txResults := getLightTransactionResultsFixture(3)

		// First batch store should succeed
		err := unittest.WithLock(t, lockManager, storage.LockInsertLightTransactionResult, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return txResultsStore.BatchStore(lctx, rw, blockID, txResults)
			})
		})
		require.NoError(t, err)

		// Second batch store with the same blockID should fail with ErrAlreadyExists
		duplicateTxResults := getLightTransactionResultsFixture(2)
		err = unittest.WithLock(t, lockManager, storage.LockInsertLightTransactionResult, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return txResultsStore.BatchStore(lctx, rw, blockID, duplicateTxResults)
			})
		})
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrAlreadyExists)

		// Verify that the original data is unchanged
		actuals, err := txResultsStore.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, len(txResults), len(actuals))
		for i := range txResults {
			assert.Equal(t, txResults[i], actuals[i])
		}
	})
}

// Test that attempting to batch store light transaction results without holding the required lock
// results in an error indicating the missing lock. The implementation should not conflate this error
// case with data for the same key already existing, ie. it should not return [storage.ErrAlreadyExists].
func TestBatchStoreLightTransactionResultsMissingLock(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		txResultsStore := store.NewLightTransactionResults(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txResults := getLightTransactionResultsFixture(3)

		// Create a context without the required lock
		lockManager := storage.NewTestingLockManager()
		lctx := lockManager.NewContext()
		defer lctx.Release()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return txResultsStore.BatchStore(lctx, rw, blockID, txResults)
		})
		require.Error(t, err)
		require.NotErrorIs(t, err, storage.ErrAlreadyExists)
		require.Contains(t, err.Error(), "lock_insert_light_transaction_result")
	})
}

// Test that attempting to batch store light transaction results while holding the wrong lock
// results in an error indicating the incorrect lock. The implementation should not conflate this error
// case with data for the same key already existing, ie. it should not return [storage.ErrAlreadyExists].
func TestBatchStoreLightTransactionResultsWrongLock(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		txResultsStore := store.NewLightTransactionResults(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txResults := getLightTransactionResultsFixture(3)

		// Try to use the wrong lock
		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return txResultsStore.BatchStore(lctx, rw, blockID, txResults)
			})
		})
		require.Error(t, err)
		require.NotErrorIs(t, err, storage.ErrAlreadyExists)
		require.Contains(t, err.Error(), "lock_insert_light_transaction_result")
	})
}

func getLightTransactionResultsFixture(n int) []flow.LightTransactionResult {
	txResults := make([]flow.LightTransactionResult, 0, n)
	for i := 0; i < n; i++ {
		expected := flow.LightTransactionResult{
			TransactionID:   unittest.IdentifierFixture(),
			Failed:          i%2 == 0,
			ComputationUsed: unittest.Uint64InRange(1, 1000),
		}
		txResults = append(txResults, expected)
	}
	return txResults
}
