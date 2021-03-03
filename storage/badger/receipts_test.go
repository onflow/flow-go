package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReceiptStoreAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		results := bstorage.NewExecutionResults(metrics, db)
		store := bstorage.NewExecutionReceipts(metrics, db, results)

		receipt := unittest.ExecutionReceiptFixture()
		err := store.Store(receipt)
		require.NoError(t, err)

		actual, err := store.ByBlockID(receipt.ExecutionResult.BlockID)
		require.NoError(t, err)

		require.Equal(t, receipt, actual[0])
	})
}

func TestReceiptStoreTwice(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		results := bstorage.NewExecutionResults(metrics, db)
		store := bstorage.NewExecutionReceipts(metrics, db, results)

		receipt := unittest.ExecutionReceiptFixture()
		err := store.Store(receipt)
		require.NoError(t, err)

		err = store.Store(receipt)
		require.NoError(t, err)
	})
}

func TestReceiptStoreTwoDifferentReceiptsShouldFail(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		results := bstorage.NewExecutionResults(metrics, db)
		store := bstorage.NewExecutionReceipts(metrics, db, results)

		block := unittest.BlockFixture()
		result1 := unittest.ExecutionResultFixture(unittest.WithBlock(&block))
		result2 := unittest.ExecutionResultFixture(unittest.WithBlock(&block))

		receipt1 := unittest.ExecutionReceiptFixture(unittest.WithResult(result1))
		receipt2 := unittest.ExecutionReceiptFixture(unittest.WithResult(result2))
		err := store.Store(receipt1)
		require.NoError(t, err)

		// we can store a different receipt, but we can't index
		// a different receipt for that block, because it will mean
		// one block has two different receipts.
		err = store.Store(receipt2)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrDataMismatch))
	})
}

func TestReceiptStoreTwoDifferentReceiptsShouldOKIfResultAreSame(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		results := bstorage.NewExecutionResults(metrics, db)
		store := bstorage.NewExecutionReceipts(metrics, db, results)

		receipt1 := unittest.ExecutionReceiptFixture()
		receipt2 := unittest.ExecutionReceiptFixture()
		receipt2.ExecutionResult = receipt1.ExecutionResult

		err := store.Store(receipt1)
		require.NoError(t, err)

		err = store.Store(receipt2)
		require.NoError(t, err)
	})
}

func TestReceiptLookupWithBlockIDAllExecutionID(t *testing.T) {
	withStore := func(t *testing.T, f func(store *bstorage.ExecutionReceipts)) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			metrics := metrics.NewNoopCollector()
			results := bstorage.NewExecutionResults(metrics, db)
			store := bstorage.NewAllExecutionReceipts(metrics, db, results)
			f(store)
		})
	}

	var empty []*flow.ExecutionReceipt

	t.Run("get empty", func(t *testing.T) {
		withStore(t, func(store *bstorage.ExecutionReceipts) {
			block := unittest.BlockFixture()
			receipts, err := store.ByBlockID(block.ID())
			require.NoError(t, err)
			require.Equal(t, empty, receipts)
		})
	})

	t.Run("store one get one", func(t *testing.T) {
		withStore(t, func(store *bstorage.ExecutionReceipts) {
			block := unittest.BlockFixture()
			receipt1 := unittest.ReceiptForBlockFixture(&block)

			err := store.Store(receipt1)
			require.NoError(t, err)

			receipts, err := store.ByBlockID(block.ID())
			require.NoError(t, err)

			require.Equal(t, []*flow.ExecutionReceipt{receipt1}, receipts)
		})
	})

	t.Run("store two for the same block", func(t *testing.T) {
		withStore(t, func(store *bstorage.ExecutionReceipts) {
			block := unittest.BlockFixture()

			executor1 := unittest.IdentifierFixture()
			executor2 := unittest.IdentifierFixture()

			receipt1 := unittest.ReceiptForBlockExecutorFixture(&block, executor1)
			receipt2 := unittest.ReceiptForBlockExecutorFixture(&block, executor2)

			err := store.Store(receipt1)
			require.NoError(t, err)

			err = store.Store(receipt2)
			require.NoError(t, err)

			receipts, err := store.ByBlockID(block.ID())
			require.NoError(t, err)

			require.ElementsMatch(t, []*flow.ExecutionReceipt{receipt1, receipt2}, receipts)
		})
	})

	t.Run("store two for different blocks", func(t *testing.T) {
		withStore(t, func(store *bstorage.ExecutionReceipts) {
			block1 := unittest.BlockFixture()
			block2 := unittest.BlockFixture()

			executor1 := unittest.IdentifierFixture()
			executor2 := unittest.IdentifierFixture()

			receipt1 := unittest.ReceiptForBlockExecutorFixture(&block1, executor1)
			receipt2 := unittest.ReceiptForBlockExecutorFixture(&block2, executor2)

			err := store.Store(receipt1)
			require.NoError(t, err)

			err = store.Store(receipt2)
			require.NoError(t, err)

			receipts, err := store.ByBlockID(block1.ID())
			require.NoError(t, err)

			require.ElementsMatch(t, []*flow.ExecutionReceipt{receipt1}, receipts)
		})
	})

	t.Run("indexing duplicated receipts should be ok", func(t *testing.T) {
		withStore(t, func(store *bstorage.ExecutionReceipts) {
			block1 := unittest.BlockFixture()

			executor1 := unittest.IdentifierFixture()
			receipt1 := unittest.ReceiptForBlockExecutorFixture(&block1, executor1)

			err := store.Store(receipt1)
			require.NoError(t, err)

			err = store.Store(receipt1)
			require.NoError(t, err)

			receipts, err := store.ByBlockID(block1.ID())
			require.NoError(t, err)

			require.ElementsMatch(t, []*flow.ExecutionReceipt{receipt1}, receipts)
		})
	})

	t.Run("indexing receipt from the same executor for same block should succeed", func(t *testing.T) {
		withStore(t, func(store *bstorage.ExecutionReceipts) {
			block1 := unittest.BlockFixture()

			executor1 := unittest.IdentifierFixture()

			receipt1 := unittest.ReceiptForBlockExecutorFixture(&block1, executor1)
			receipt2 := unittest.ReceiptForBlockExecutorFixture(&block1, executor1)

			err := store.Store(receipt1)
			require.NoError(t, err)

			err = store.Store(receipt2)
			require.NoError(t, err)

			receipts, err := store.ByBlockID(block1.ID())
			require.NoError(t, err)

			require.ElementsMatch(t, []*flow.ExecutionReceipt{receipt1, receipt2}, receipts)
		})
	})
}
