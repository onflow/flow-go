package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestResultStoreAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewExecutionResults(metrics, db)

		result := unittest.ExecutionResultFixture()
		blockID := unittest.IdentifierFixture()
		err := store.Store(result)
		require.NoError(t, err)

		err = store.Index(blockID, result.ID())
		require.NoError(t, err)

		actual, err := store.ByBlockID(blockID)
		require.NoError(t, err)

		require.Equal(t, result, actual)
	})
}

func TestResultStoreTwice(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewExecutionResults(metrics, db)

		result := unittest.ExecutionResultFixture()
		blockID := unittest.IdentifierFixture()
		err := store.Store(result)
		require.NoError(t, err)

		err = store.Index(blockID, result.ID())
		require.NoError(t, err)

		err = store.Store(result)
		require.NoError(t, err)

		err = store.Index(blockID, result.ID())
		require.NoError(t, err)
	})
}

func TestResultStoreTwoDifferentResultsShouldFail(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewExecutionResults(metrics, db)

		result1 := unittest.ExecutionResultFixture()
		result2 := unittest.ExecutionResultFixture()
		blockID := unittest.IdentifierFixture()
		err := store.Store(result1)
		require.NoError(t, err)

		err = store.Index(blockID, result1.ID())
		require.NoError(t, err)

		// we can store a different result, but we can't index
		// a different result for that block, because it will mean
		// one block has two different results.
		err = store.Store(result2)
		require.NoError(t, err)

		err = store.Index(blockID, result2.ID())
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrDataMismatch))
	})
}

func TestResultStoreForceIndexOverridesMapping(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewExecutionResults(metrics, db)

		result1 := unittest.ExecutionResultFixture()
		result2 := unittest.ExecutionResultFixture()
		blockID := unittest.IdentifierFixture()
		err := store.Store(result1)
		require.NoError(t, err)
		err = store.Index(blockID, result1.ID())
		require.NoError(t, err)

		err = store.Store(result2)
		require.NoError(t, err)

		// force index
		err = store.ForceIndex(blockID, result2.ID())
		require.NoError(t, err)

		// retrieve index to make sure it points to second ER now
		byBlockID, err := store.ByBlockID(blockID)

		require.Equal(t, result2, byBlockID)
		require.NoError(t, err)
	})
}

func TestBatchReindex(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewExecutionResults(metrics, db)

		blockID1 := unittest.IdentifierFixture()

		result1 := unittest.ExecutionResultFixture()
		result2 := unittest.ExecutionResultFixture()

		batch1 := bstorage.NewBatch(db)
		require.NoError(t, store.BatchStore(result1, batch1))
		require.NoError(t, store.BatchIndex(blockID1, result1.ID(), false, batch1))
		require.NoError(t, batch1.Flush())

		batch2 := bstorage.NewBatch(db)
		require.NoError(t, store.BatchStore(result2, batch2))
		require.NoError(t, store.BatchIndex(blockID1, result2.ID(), true, batch2))
		require.NoError(t, batch2.Flush())

		// retrieve index to make sure it points to second ER now
		byBlockID, err := store.ByBlockID(blockID1)

		require.Equal(t, result2, byBlockID)
		require.NoError(t, err)
	})
}
