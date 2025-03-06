package store_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMyExecutionReceiptsStorage(t *testing.T) {
	withStore := func(t *testing.T, f func(store1 *store.MyExecutionReceipts, db storage.DB)) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			metrics := metrics.NewNoopCollector()
			results := store.NewExecutionResults(metrics, db)
			receipts := store.NewExecutionReceipts(metrics, db, results, 100)
			store1 := store.NewMyExecutionReceipts(metrics, db, receipts)

			f(store1, db)
		})
	}

	t.Run("store1 one get one", func(t *testing.T) {
		withStore(t, func(store1 *store.MyExecutionReceipts, db storage.DB) {
			block := unittest.BlockFixture()
			receipt1 := unittest.ReceiptForBlockFixture(&block)

			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store1.BatchStoreMyReceipt(receipt1, rw)
			})
			require.NoError(t, err)

			actual, err := store1.MyReceipt(block.ID())
			require.NoError(t, err)

			require.Equal(t, receipt1, actual)
		})
	})

	t.Run("store1 same for the same block", func(t *testing.T) {
		withStore(t, func(store1 *store.MyExecutionReceipts, db storage.DB) {
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

	t.Run("store different receipt for same block should fail", func(t *testing.T) {
		withStore(t, func(store *store.MyExecutionReceipts, db storage.DB) {
			block := unittest.BlockFixture()

			executor1 := unittest.IdentifierFixture()
			executor2 := unittest.IdentifierFixture()

			receipt1 := unittest.ReceiptForBlockExecutorFixture(&block, executor1)
			receipt2 := unittest.ReceiptForBlockExecutorFixture(&block, executor2)

			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store.BatchStoreMyReceipt(receipt1, rw)
			})
			require.NoError(t, err)

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store.BatchStoreMyReceipt(receipt2, rw)
			})

			require.Error(t, err)
			require.Contains(t, err.Error(), "different receipt")
		})
	})

	t.Run("concurrent store different receipt for same block should fail", func(t *testing.T) {
		withStore(t, func(store *store.MyExecutionReceipts, db storage.DB) {
			block := unittest.BlockFixture()

			executor1 := unittest.IdentifierFixture()
			executor2 := unittest.IdentifierFixture()

			receipt1 := unittest.ReceiptForBlockExecutorFixture(&block, executor1)
			receipt2 := unittest.ReceiptForBlockExecutorFixture(&block, executor2)

			var wg sync.WaitGroup
			errChan := make(chan error, 2)

			wg.Add(2)

			go func() {
				defer wg.Done()
				err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return store.BatchStoreMyReceipt(receipt1, rw)
				})
				errChan <- err
			}()

			go func() {
				defer wg.Done()
				err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return store.BatchStoreMyReceipt(receipt2, rw)
				})
				errChan <- err
			}()

			wg.Wait()
			close(errChan)

			var errCount int
			for err := range errChan {
				if err != nil {
					errCount++
					require.Contains(t, err.Error(), "different receipt")
				}
			}

			require.Equal(t, 1, errCount, "Exactly one of the operations should fail")
		})
	})

	t.Run("concurrent store of 10 different receipts for different blocks should succeed", func(t *testing.T) {
		withStore(t, func(store *store.MyExecutionReceipts, db storage.DB) {
			var wg sync.WaitGroup
			errChan := make(chan error, 10)

			// Store receipts concurrently
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()

					block := unittest.BlockFixture() // Each iteration gets a new block
					executor := unittest.IdentifierFixture()
					receipt := unittest.ReceiptForBlockExecutorFixture(&block, executor)

					err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return store.BatchStoreMyReceipt(receipt, rw)
					})

					errChan <- err
				}(i)
			}

			wg.Wait()
			close(errChan)

			// Verify all succeeded
			for err := range errChan {
				require.NoError(t, err, "All receipts should be stored successfully")
			}
		})
	})
}
