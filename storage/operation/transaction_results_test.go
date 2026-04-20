package operation_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInsertAndIndexTransactionResults verifies the insertion and indexing of transaction results
// on the happy path.
func TestInsertAndIndexTransactionResults(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		blockID := unittest.IdentifierFixture()

		// Create test transaction results
		transactionResults := unittest.TransactionResultsFixture(3)

		// Test successful insertion and indexing
		err := unittest.WithLock(t, lockManager, storage.LockInsertAndIndexTxResult, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertAndIndexTransactionResults(lctx, rw, blockID, transactionResults)
			})
		})
		require.NoError(t, err)

		// Verify that transaction results can be retrieved by transaction ID
		for i, expected := range transactionResults {
			var actual flow.TransactionResult
			err = operation.RetrieveTransactionResult(db.Reader(), blockID, expected.TransactionID, &actual)
			require.NoError(t, err)
			assert.Equal(t, expected, actual)

			// Verify that transaction results can be retrieved by index
			var actualByIndex flow.TransactionResult
			err = operation.RetrieveTransactionResultByIndex(db.Reader(), blockID, uint32(i), &actualByIndex)
			require.NoError(t, err)
			assert.Equal(t, expected, actualByIndex)
		}

		// Verify that all transaction results can be retrieved using the index
		var retrievedResults []flow.TransactionResult
		err = operation.LookupTransactionResultsByBlockIDUsingIndex(db.Reader(), blockID, &retrievedResults)
		require.NoError(t, err)
		assert.Len(t, retrievedResults, len(transactionResults))

		// Verify the order matches the original order
		for i, expected := range transactionResults {
			assert.Equal(t, expected, retrievedResults[i])
		}
	})
}

// TestInsertAndIndexTransactionResults_AlreadyExists verifies that attempting to insert
// transaction results for a block that already has results returns `storage.ErrAlreadyExists`.
func TestInsertAndIndexTransactionResults_AlreadyExists(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		blockID := unittest.IdentifierFixture()

		// Create initial transaction results
		initialResults := unittest.TransactionResultsFixture(1)

		// Insert initial results
		err := unittest.WithLock(t, lockManager, storage.LockInsertAndIndexTxResult, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertAndIndexTransactionResults(lctx, rw, blockID, initialResults)
			})
		})
		require.NoError(t, err)

		// Try to insert results for the same block again - should fail
		duplicateResults := unittest.TransactionResultsFixture(1)

		err = unittest.WithLock(t, lockManager, storage.LockInsertAndIndexTxResult, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertAndIndexTransactionResults(lctx, rw, blockID, duplicateResults)
			})
		})
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
	})
}

// TestInsertAndIndexTransactionResults_NoLock verifies that attempting to insert
// transaction results while holding no lock returns an error.
func TestInsertAndIndexTransactionResults_NoLock(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		blockID := unittest.IdentifierFixture()
		transactionResults := unittest.TransactionResultsFixture(1)

		// Try to insert without holding the required lock - should fail
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			// Create a context without the required lock
			lockManager := storage.NewTestingLockManager()
			lctx := lockManager.NewContext()
			defer lctx.Release()

			return operation.InsertAndIndexTransactionResults(lctx, rw, blockID, transactionResults)
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "InsertTransactionResult requires LockInsertAndIndexTxResult to be held")
	})
}

// TestInsertAndIndexTransactionResults_EmptyResults verifies that inserting
// an empty slice of transaction is effectively a no-op.
func TestInsertAndIndexTransactionResults_EmptyResults(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		blockID := unittest.IdentifierFixture()

		// Test with empty transaction results
		emptyResults := []flow.TransactionResult{}

		err := unittest.WithLock(t, lockManager, storage.LockInsertAndIndexTxResult, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertAndIndexTransactionResults(lctx, rw, blockID, emptyResults)
			})
		})
		require.NoError(t, err)

		// Verify that no results are stored
		var retrievedResults []flow.TransactionResult
		err = operation.LookupTransactionResultsByBlockIDUsingIndex(db.Reader(), blockID, &retrievedResults)
		require.NoError(t, err)
		assert.Len(t, retrievedResults, 0)
	})
}

// TestInsertAndIndexTransactionResults_SingleResult verifies that inserting
// a batch containing only a single transaction works.
func TestInsertAndIndexTransactionResults_SingleResult(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		blockID := unittest.IdentifierFixture()

		// Test with single transaction result
		singleResult := unittest.TransactionResultsFixture(1)

		err := unittest.WithLock(t, lockManager, storage.LockInsertAndIndexTxResult, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertAndIndexTransactionResults(lctx, rw, blockID, singleResult)
			})
		})
		require.NoError(t, err)

		// Verify the single result can be retrieved
		var retrievedResults []flow.TransactionResult
		err = operation.LookupTransactionResultsByBlockIDUsingIndex(db.Reader(), blockID, &retrievedResults)
		require.NoError(t, err)
		assert.Len(t, retrievedResults, 1)
		assert.Equal(t, singleResult[0], retrievedResults[0])
	})
}

// TestInsertAndIndexTransactionResults_MultipleResultsWithSameTransactionID verifies that inserting
// a block with more than one result for the same transaction IDs is handled. This is an important BFT
// edge case, which can occur if we have a cluster collector with byzantine supermajority (quite
// small but non-vanishing probability, which we still have to account for).
func TestInsertAndIndexTransactionResults_MultipleResultsWithSameTransactionID(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		blockID := unittest.IdentifierFixture()

		// Create transaction results with the same transaction ID (duplicate transactions in block)
		transactionID := unittest.IdentifierFixture()
		results := []flow.TransactionResult{
			{
				TransactionID:   transactionID,
				ErrorMessage:    "error1",
				ComputationUsed: 100,
				MemoryUsed:      200,
			},
			{
				TransactionID:   transactionID,
				ErrorMessage:    "error2",
				ComputationUsed: 150,
				MemoryUsed:      250,
			},
		}

		err := unittest.WithLock(t, lockManager, storage.LockInsertAndIndexTxResult, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertAndIndexTransactionResults(lctx, rw, blockID, results)
			})
		})
		require.NoError(t, err)

		// Verify both results are stored and can be retrieved by index
		for i, expected := range results {
			var actual flow.TransactionResult
			err = operation.RetrieveTransactionResultByIndex(db.Reader(), blockID, uint32(i), &actual)
			require.NoError(t, err)
			assert.Equal(t, expected, actual)
		}

		// Verify that lookup by transaction ID returns the last stored result (due to overwrite)
		var actual flow.TransactionResult
		err = operation.RetrieveTransactionResult(db.Reader(), blockID, transactionID, &actual)
		require.NoError(t, err)
		assert.Equal(t, results[1], actual) // Should be the last one stored
	})
}

// TestRetrieveAllTxResultsForBlock verifies the working of persisting, indexing and retrieving
// [flow.LightTransactionResult] by block, transaction ID, and transaction index.
func TestRetrieveAllTxResultsForBlock(t *testing.T) {
	t.Run("looking up transaction results for unknown block yields empty list", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			unknownBlockID := unittest.IdentifierFixture()
			transactionResults := make([]flow.LightTransactionResult, 0)

			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.LookupLightTransactionResultsByBlockIDUsingIndex(rw.GlobalReader(), unknownBlockID, &transactionResults)
			})
			require.NoError(t, err)
			require.Empty(t, transactionResults)
		})
	})
}

// TestInsertAndIndexTransactionResultErrorMessages verifies that error messages can be inserted,
// retrieved by transaction ID and index, and that duplicate inserts are rejected.
func TestInsertAndIndexTransactionResultErrorMessages(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			lockManager := storage.NewTestingLockManager()
			blockID := unittest.IdentifierFixture()
			errorMessages := unittest.TransactionResultErrorMessagesFixture(3)

			err := unittest.WithLock(t, lockManager, storage.LockInsertTransactionResultErrMessage, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.InsertAndIndexTransactionResultErrorMessages(lctx, rw, blockID, errorMessages)
				})
			})
			require.NoError(t, err)

			// Verify retrieval by transaction ID.
			for _, expected := range errorMessages {
				var actual flow.TransactionResultErrorMessage
				err = operation.RetrieveTransactionResultErrorMessage(db.Reader(), blockID, expected.TransactionID, &actual)
				require.NoError(t, err)
				assert.Equal(t, expected, actual)
			}

			// Verify retrieval by index.
			for i, expected := range errorMessages {
				var actual flow.TransactionResultErrorMessage
				err = operation.RetrieveTransactionResultErrorMessageByIndex(db.Reader(), blockID, uint32(i), &actual)
				require.NoError(t, err)
				assert.Equal(t, expected, actual)
			}

			// Verify bulk retrieval.
			var retrieved []flow.TransactionResultErrorMessage
			err = operation.LookupTransactionResultErrorMessagesByBlockIDUsingIndex(db.Reader(), blockID, &retrieved)
			require.NoError(t, err)
			assert.Equal(t, errorMessages, retrieved)
		})
	})

	t.Run("returns ErrAlreadyExists on duplicate insert", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			lockManager := storage.NewTestingLockManager()
			blockID := unittest.IdentifierFixture()
			errorMessages := unittest.TransactionResultErrorMessagesFixture(1)

			insert := func() error {
				return unittest.WithLock(t, lockManager, storage.LockInsertTransactionResultErrMessage, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return operation.InsertAndIndexTransactionResultErrorMessages(lctx, rw, blockID, errorMessages)
					})
				})
			}

			err := insert()
			require.NoError(t, err)

			err = insert()
			require.ErrorIs(t, err, storage.ErrAlreadyExists)

			// Verify that the original error messages are still there and unchanged
			for _, expected := range errorMessages {
				var actual flow.TransactionResultErrorMessage
				err = operation.RetrieveTransactionResultErrorMessage(db.Reader(), blockID, expected.TransactionID, &actual)
				require.NoError(t, err)
				assert.Equal(t, expected, actual)
			}
		})
	})
}

// TestTransactionResultErrorMessagesExists verifies the existence check for tx result error messages.
func TestTransactionResultErrorMessagesExists(t *testing.T) {
	t.Run("returns false for unknown block", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			blockID := unittest.IdentifierFixture()
			var exists bool
			err := operation.TransactionResultErrorMessagesExists(db.Reader(), blockID, &exists)
			require.NoError(t, err)
			require.False(t, exists)
		})
	})

	t.Run("returns true after inserting error messages", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			lockManager := storage.NewTestingLockManager()
			blockID := unittest.IdentifierFixture()
			errorMessages := unittest.TransactionResultErrorMessagesFixture(2)

			err := unittest.WithLock(t, lockManager, storage.LockInsertTransactionResultErrMessage, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.InsertAndIndexTransactionResultErrorMessages(lctx, rw, blockID, errorMessages)
				})
			})
			require.NoError(t, err)

			var exists bool
			err = operation.TransactionResultErrorMessagesExists(db.Reader(), blockID, &exists)
			require.NoError(t, err)
			require.True(t, exists)
		})
	})

	t.Run("returns false for a different block", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			lockManager := storage.NewTestingLockManager()
			blockID := unittest.IdentifierFixture()
			otherBlockID := unittest.IdentifierFixture()
			errorMessages := unittest.TransactionResultErrorMessagesFixture(1)

			err := unittest.WithLock(t, lockManager, storage.LockInsertTransactionResultErrMessage, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.InsertAndIndexTransactionResultErrorMessages(lctx, rw, blockID, errorMessages)
				})
			})
			require.NoError(t, err)

			var exists bool
			err = operation.TransactionResultErrorMessagesExists(db.Reader(), otherBlockID, &exists)
			require.NoError(t, err)
			require.False(t, exists)
		})
	})
}
