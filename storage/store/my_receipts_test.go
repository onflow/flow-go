package store_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMyExecutionReceiptsStorage(t *testing.T) {
	withStore := func(t *testing.T, f func(storage.MyExecutionReceipts, storage.ExecutionResults, storage.ExecutionReceipts, storage.DB, lockctx.Manager)) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			lockManager := storage.NewTestingLockManager()
			metrics := metrics.NewNoopCollector()
			results := store.NewExecutionResults(metrics, db)
			receipts := store.NewExecutionReceipts(metrics, db, results, 100)
			myReceipts := store.NewMyExecutionReceipts(metrics, db, receipts)

			f(myReceipts, results, receipts, db, lockManager)
		})
	}

	t.Run("myReceipts store and retrieve from different storage layers", func(t *testing.T) {
		withStore(t, func(myReceipts storage.MyExecutionReceipts, results storage.ExecutionResults, receipts storage.ExecutionReceipts, db storage.DB, lockManager lockctx.Manager) {
			block := unittest.BlockFixture()
			receipt1 := unittest.ReceiptForBlockFixture(block)

			// STEP 1: Store receipt
			err := unittest.WithLock(t, lockManager, storage.LockInsertMyReceipt, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return myReceipts.BatchStoreMyReceipt(lctx, receipt1, rw)
				})
			})
			require.NoError(t, err)

			// STEP 2: Retrieve from different storage layers
			// MyExecutionReceipts delegates the storage of the receipt to the more generic storage.ExecutionReceipts and storage.ExecutionResults,
			// which is also used by the consensus follower to store execution receipts & results that are incorporated into blocks.
			// After storing my receipts, we check that the result and receipt can also be retrieved from the lower-level generic storage layers.
			actual, err := myReceipts.MyReceipt(block.ID())
			require.NoError(t, err)
			require.Equal(t, receipt1, actual)

			actualReceipt, err := receipts.ByID(receipt1.ID()) // generic receipts storage
			require.NoError(t, err)
			require.Equal(t, receipt1, actualReceipt)

			actualResult, err := results.ByID(receipt1.ExecutionResult.ID()) // generic results storage
			require.NoError(t, err)
			require.Equal(t, receipt1.ExecutionResult, *actualResult)
		})
	})

	t.Run("myReceipts store identical receipt for the same block", func(t *testing.T) {
		withStore(t, func(myReceipts storage.MyExecutionReceipts, _ storage.ExecutionResults, _ storage.ExecutionReceipts, db storage.DB, lockManager lockctx.Manager) {
			block := unittest.BlockFixture()
			receipt1 := unittest.ReceiptForBlockFixture(block)

			err := unittest.WithLock(t, lockManager, storage.LockInsertMyReceipt, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return myReceipts.BatchStoreMyReceipt(lctx, receipt1, rw)
				})
			})
			require.NoError(t, err)

			err = unittest.WithLock(t, lockManager, storage.LockInsertMyReceipt, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return myReceipts.BatchStoreMyReceipt(lctx, receipt1, rw)
				})
			})
			require.NoError(t, err, "`BatchStoreMyReceipt` should be idempotent for the same receipt")
		})
	})

	t.Run("store different receipt for same block should fail", func(t *testing.T) {
		withStore(t, func(myReceipts storage.MyExecutionReceipts, results storage.ExecutionResults, receipts storage.ExecutionReceipts, db storage.DB, lockManager lockctx.Manager) {
			block := unittest.BlockFixture()

			executor1 := unittest.IdentifierFixture()
			executor2 := unittest.IdentifierFixture()

			receipt1 := unittest.ReceiptForBlockExecutorFixture(block, executor1)
			receipt2 := unittest.ReceiptForBlockExecutorFixture(block, executor2)

			err := unittest.WithLock(t, lockManager, storage.LockInsertMyReceipt, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return myReceipts.BatchStoreMyReceipt(lctx, receipt1, rw)
				})
			})
			require.NoError(t, err)

			err = unittest.WithLock(t, lockManager, storage.LockInsertMyReceipt, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return myReceipts.BatchStoreMyReceipt(lctx, receipt2, rw)
				})
			})
			require.Error(t, err)
			require.ErrorIs(t, err, storage.ErrDataMismatch)
		})
	})

	t.Run("concurrent store different receipt for same block should fail", func(t *testing.T) {
		withStore(t, func(myReceipts storage.MyExecutionReceipts, results storage.ExecutionResults, receipts storage.ExecutionReceipts, db storage.DB, lockManager lockctx.Manager) {
			block := unittest.BlockFixture()

			executor1 := unittest.IdentifierFixture()
			executor2 := unittest.IdentifierFixture()

			receipt1 := unittest.ReceiptForBlockExecutorFixture(block, executor1)
			receipt2 := unittest.ReceiptForBlockExecutorFixture(block, executor2)

			var startSignal sync.WaitGroup // goroutines attempting store operations will wait for this signal to start concurrently
			startSignal.Add(1)             // expecting one signal from the main thread to start both goroutines
			var doneSinal sync.WaitGroup   // the main thread will wait on this for both goroutines to finish
			doneSinal.Add(2)               // expecting two goroutines to signal finish
			errChan := make(chan error, 2)

			go func() {
				startSignal.Wait()
				err := unittest.WithLock(t, lockManager, storage.LockInsertMyReceipt, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return myReceipts.BatchStoreMyReceipt(lctx, receipt1, rw)
					})
				})
				errChan <- err
				doneSinal.Done()
			}()

			go func() {
				startSignal.Wait()
				err := unittest.WithLock(t, lockManager, storage.LockInsertMyReceipt, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return myReceipts.BatchStoreMyReceipt(lctx, receipt2, rw)
					})
				})
				errChan <- err
				doneSinal.Done()
			}()

			startSignal.Done() // start both goroutines
			doneSinal.Wait()   // wait for both goroutines to finish
			close(errChan)

			// Check that one of the Index operations succeeded and the other failed
			var errCount int
			for err := range errChan {
				if err != nil {
					errCount++
					require.Contains(t, err.Error(), "data for key is different")
				}
			}
			require.Equal(t, 1, errCount, "Exactly one of the operations should fail")
		})
	})

	t.Run("concurrent store of 10 different receipts for different blocks should succeed", func(t *testing.T) {
		withStore(t, func(myReceipts storage.MyExecutionReceipts, results storage.ExecutionResults, receipts storage.ExecutionReceipts, db storage.DB, lockManager lockctx.Manager) {
			var startSignal sync.WaitGroup // goroutines attempting store operations will wait for this signal to start concurrently
			startSignal.Add(1)             // expecting one signal from the main thread to start both goroutines
			var doneSinal sync.WaitGroup   // the main thread will wait on this for goroutines attempting store operations to finish
			errChan := make(chan error, 10)

			// Store receipts concurrently
			for i := 0; i < 10; i++ {
				doneSinal.Add(1)
				go func(i int) {
					block := unittest.BlockFixture() // Each iteration gets a new block
					executor := unittest.IdentifierFixture()
					receipt := unittest.ReceiptForBlockExecutorFixture(block, executor)

					startSignal.Wait()
					err := unittest.WithLock(t, lockManager, storage.LockInsertMyReceipt, func(lctx lockctx.Context) error {
						return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
							return myReceipts.BatchStoreMyReceipt(lctx, receipt, rw)
						})
					})
					errChan <- err
					require.NoError(t, err)
					doneSinal.Done()
				}(i)
			}

			startSignal.Done() // start both goroutines
			doneSinal.Wait()   // wait for both goroutines to finish
			close(errChan)

			// Verify all succeeded
			for err := range errChan {
				require.NoError(t, err, "All receipts should be stored successfully")
			}
		})
	})

	t.Run("store and remove", func(t *testing.T) {
		withStore(t, func(myReceipts storage.MyExecutionReceipts, results storage.ExecutionResults, receipts storage.ExecutionReceipts, db storage.DB, lockManager lockctx.Manager) {
			block := unittest.BlockFixture()
			receipt1 := unittest.ReceiptForBlockFixture(block)

			err := unittest.WithLock(t, lockManager, storage.LockInsertMyReceipt, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return myReceipts.BatchStoreMyReceipt(lctx, receipt1, rw)
				})
			})
			require.NoError(t, err)

			actual, err := myReceipts.MyReceipt(block.ID())
			require.NoError(t, err)

			require.Equal(t, receipt1, actual)

			// Check after storing my receipts, the result and receipt are stored
			actualReceipt, err := receipts.ByID(receipt1.ID())
			require.NoError(t, err)
			require.Equal(t, receipt1, actualReceipt)

			actualResult, err := results.ByID(receipt1.ExecutionResult.ID())
			require.NoError(t, err)
			require.Equal(t, receipt1.ExecutionResult, *actualResult)

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return myReceipts.BatchRemoveIndexByBlockID(block.ID(), rw)
			})
			require.NoError(t, err)

			_, err = myReceipts.MyReceipt(block.ID())
			require.True(t, errors.Is(err, storage.ErrNotFound))
		})
	})
}

func TestMyExecutionReceiptsStorageMultipleStoreInSameBatch(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		results := store.NewExecutionResults(metrics, db)
		receipts := store.NewExecutionReceipts(metrics, db, results, 100)
		myReceipts := store.NewMyExecutionReceipts(metrics, db, receipts)

		block := unittest.BlockFixture()
		receipt1 := unittest.ReceiptForBlockFixture(block)
		receipt2 := unittest.ReceiptForBlockFixture(block)

		err := unittest.WithLock(t, lockManager, storage.LockInsertMyReceipt, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				err := myReceipts.BatchStoreMyReceipt(lctx, receipt1, rw)
				if err != nil {
					return err
				}
				return myReceipts.BatchStoreMyReceipt(lctx, receipt2, rw)
			})
		})
		require.NoError(t, err)
	})
}
