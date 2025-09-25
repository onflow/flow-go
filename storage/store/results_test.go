package store_test

import (
	"errors"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestResultStoreAndRetrieve(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store1 := store.NewExecutionResults(metrics, db)

		result := unittest.ExecutionResultFixture()
		blockID := unittest.IdentifierFixture()
		err := store1.Store(result)
		require.NoError(t, err)

		unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store1.BatchIndex(lctx, rw, blockID, result.ID())
			})
		})

		actual, err := store1.ByBlockID(blockID)
		require.NoError(t, err)

		require.Equal(t, result, actual)
	})
}

func TestResultStoreTwice(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store1 := store.NewExecutionResults(metrics, db)

		result := unittest.ExecutionResultFixture()
		blockID := unittest.IdentifierFixture()
		err := store1.Store(result)
		require.NoError(t, err)

		unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store1.BatchIndex(lctx, rw, blockID, result.ID())
			})
		})

		err = store1.Store(result)
		require.NoError(t, err)

		unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store1.BatchIndex(lctx, rw, blockID, result.ID())
			})
		})
	})
}

func TestResultBatchStoreTwice(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store1 := store.NewExecutionResults(metrics, db)

		result := unittest.ExecutionResultFixture()
		blockID := unittest.IdentifierFixture()

		unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(batch storage.ReaderBatchWriter) error {
				err := store1.BatchStore(result, batch)
				require.NoError(t, err)

				err = store1.BatchIndex(lctx, batch, blockID, result.ID())
				require.NoError(t, err)
				return nil
			})
		})

		unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(batch storage.ReaderBatchWriter) error {
				err := store1.BatchStore(result, batch)
				require.NoError(t, err)

				err = store1.BatchIndex(lctx, batch, blockID, result.ID())
				require.NoError(t, err)

				return nil
			})
		})
	})
}

func TestResultStoreTwoDifferentResultsShouldFail(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store1 := store.NewExecutionResults(metrics, db)

		result1 := unittest.ExecutionResultFixture()
		result2 := unittest.ExecutionResultFixture()
		blockID := unittest.IdentifierFixture()
		err := store1.Store(result1)
		require.NoError(t, err)

		unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store1.BatchIndex(lctx, rw, blockID, result1.ID())
			})
		})

		// we can store1 a different result, but we can't index
		// a different result for that block, because it will mean
		// one block has two different results.
		err = store1.Store(result2)
		require.NoError(t, err)

		var indexErr error
		unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				indexErr = store1.BatchIndex(lctx, rw, blockID, result2.ID())
				return nil
			})
		})
		require.Error(t, indexErr)
		require.True(t, errors.Is(indexErr, storage.ErrDataMismatch))
	})
}
