package store_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMyExecutionReceiptsStorage(t *testing.T) {
	withStore := func(t *testing.T, f func(storage.MyExecutionReceipts, storage.ExecutionResults, storage.ExecutionReceipts, storage.DB)) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			metrics := metrics.NewNoopCollector()
			results := store.NewExecutionResults(metrics, db)
			receipts := store.NewExecutionReceipts(metrics, db, results, 100)
			store1 := store.NewMyExecutionReceipts(metrics, db, receipts)

			f(store1, results, receipts, db)
		})
	}

	t.Run("store1 one get one", func(t *testing.T) {
		withStore(t, func(store1 storage.MyExecutionReceipts, results storage.ExecutionResults, receipts storage.ExecutionReceipts, db storage.DB) {
			block := unittest.BlockFixture()
			receipt1 := unittest.ReceiptForBlockFixture(&block)

			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store1.BatchStoreMyReceipt(receipt1, rw)
			})
			require.NoError(t, err)

			actual, err := store1.MyReceipt(block.ID())
			require.NoError(t, err)

			require.Equal(t, receipt1, actual)

			// Check after storing my receipts, the result and receipt are stored
			actualReceipt, err := receipts.ByID(receipt1.ID())
			require.NoError(t, err)
			require.Equal(t, receipt1, actualReceipt)

			actualResult, err := results.ByID(receipt1.ExecutionResult.ID())
			require.NoError(t, err)
			require.Equal(t, receipt1.ExecutionResult, *actualResult)
		})
	})

	t.Run("store1 same for the same block", func(t *testing.T) {
		withStore(t, func(store1 storage.MyExecutionReceipts, _ storage.ExecutionResults, _ storage.ExecutionReceipts, db storage.DB) {
			block := unittest.BlockFixture()

			receipt1 := unittest.ReceiptForBlockFixture(&block)

			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store1.BatchStoreMyReceipt(receipt1, rw)
			})
			require.NoError(t, err)

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store1.BatchStoreMyReceipt(receipt1, rw)
			})
			require.NoError(t, err)
		})
	})
}
