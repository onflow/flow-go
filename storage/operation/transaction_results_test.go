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
