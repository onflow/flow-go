package store_test

import (
	"fmt"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
)

func TestStoringTransactionResultErrorMessages(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		txErrMsgStore := store.NewTransactionResultErrorMessages(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()

		// test db Exists by block id
		exists, err := txErrMsgStore.Exists(blockID)
		require.NoError(t, err)
		require.False(t, exists)

		// check retrieving by ByBlockID
		messages, err := txErrMsgStore.ByBlockID(blockID)
		require.NoError(t, err)
		require.Nil(t, messages)

		txErrorMessages := make([]flow.TransactionResultErrorMessage, 0)
		for i := 0; i < 10; i++ {
			expected := flow.TransactionResultErrorMessage{
				TransactionID: unittest.IdentifierFixture(),
				ErrorMessage:  fmt.Sprintf("a runtime error %d", i),
				ExecutorID:    unittest.IdentifierFixture(),
				Index:         rand.Uint32(),
			}
			txErrorMessages = append(txErrorMessages, expected)
		}
		err = unittest.WithLock(t, lockManager, storage.LockInsertTransactionResultErrMessage, func(lctx lockctx.Context) error {
			return txErrMsgStore.Store(lctx, blockID, txErrorMessages)
		})
		require.NoError(t, err)

		// test db Exists by block id
		exists, err = txErrMsgStore.Exists(blockID)
		require.NoError(t, err)
		require.True(t, exists)

		// check retrieving by ByBlockIDTransactionID
		for _, txErrorMessage := range txErrorMessages {
			actual, err := txErrMsgStore.ByBlockIDTransactionID(blockID, txErrorMessage.TransactionID)
			require.NoError(t, err)
			assert.Equal(t, txErrorMessage, *actual)
		}

		// check retrieving by ByBlockIDTransactionIndex
		for _, txErrorMessage := range txErrorMessages {
			actual, err := txErrMsgStore.ByBlockIDTransactionIndex(blockID, txErrorMessage.Index)
			require.NoError(t, err)
			assert.Equal(t, txErrorMessage, *actual)
		}

		// check retrieving by ByBlockID
		actual, err := txErrMsgStore.ByBlockID(blockID)
		require.NoError(t, err)
		assert.Equal(t, txErrorMessages, actual)

		// test loading from database
		newStore := store.NewTransactionResultErrorMessages(metrics, db, 1000)
		for _, txErrorMessage := range txErrorMessages {
			actual, err := newStore.ByBlockIDTransactionID(blockID, txErrorMessage.TransactionID)
			require.NoError(t, err)
			assert.Equal(t, txErrorMessage, *actual)
		}

		// check retrieving by index from both cache and db
		for i, txErrorMessage := range txErrorMessages {
			actual, err := txErrMsgStore.ByBlockIDTransactionIndex(blockID, txErrorMessage.Index)
			require.NoError(t, err)
			assert.Equal(t, txErrorMessages[i], *actual)

			actual, err = newStore.ByBlockIDTransactionIndex(blockID, txErrorMessage.Index)
			require.NoError(t, err)
			assert.Equal(t, txErrorMessages[i], *actual)
		}
	})
}

func TestReadingNotStoreTransactionResultErrorMessage(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		txErrMsgStore := store.NewTransactionResultErrorMessages(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()
		txIndex := rand.Uint32()

		_, err := txErrMsgStore.ByBlockIDTransactionID(blockID, txID)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		_, err = txErrMsgStore.ByBlockIDTransactionIndex(blockID, txIndex)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}

// Test that attempting to batch store transaction result error messages for a block ID that already exists
// results in a [storage.ErrAlreadyExists] error, and that the original messages remain unchanged.
func TestBatchStoreTransactionResultErrorMessagesErrAlreadyExists(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		st := store.NewTransactionResultErrorMessages(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txResultErrMsgs := make([]flow.TransactionResultErrorMessage, 0)
		for i := 0; i < 3; i++ {
			expected := flow.TransactionResultErrorMessage{
				TransactionID: unittest.IdentifierFixture(),
				ErrorMessage:  fmt.Sprintf("a runtime error %d", i),
				Index:         uint32(i),
				ExecutorID:    unittest.IdentifierFixture(),
			}
			txResultErrMsgs = append(txResultErrMsgs, expected)
		}

		// First batch store should succeed
		err := unittest.WithLock(t, lockManager, storage.LockInsertTransactionResultErrMessage, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return st.BatchStore(lctx, rw, blockID, txResultErrMsgs)
			})
		})
		require.NoError(t, err)

		// Second batch store with the same blockID should fail with ErrAlreadyExists
		duplicateTxResultErrMsgs := make([]flow.TransactionResultErrorMessage, 0)
		for i := 0; i < 2; i++ {
			expected := flow.TransactionResultErrorMessage{
				TransactionID: unittest.IdentifierFixture(),
				ErrorMessage:  fmt.Sprintf("duplicate error %d", i),
				Index:         uint32(i),
				ExecutorID:    unittest.IdentifierFixture(),
			}
			duplicateTxResultErrMsgs = append(duplicateTxResultErrMsgs, expected)
		}

		err = unittest.WithLock(t, lockManager, storage.LockInsertTransactionResultErrMessage, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return st.BatchStore(lctx, rw, blockID, duplicateTxResultErrMsgs)
			})
		})
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrAlreadyExists)

		// Verify that the original transaction result error messages are still there and unchanged
		for _, txResultErrMsg := range txResultErrMsgs {
			actual, err := st.ByBlockIDTransactionID(blockID, txResultErrMsg.TransactionID)
			require.NoError(t, err)
			assert.Equal(t, txResultErrMsg, *actual)
		}

		// Verify that the duplicate transaction result error messages were not stored
		for _, txResultErrMsg := range duplicateTxResultErrMsgs {
			_, err := st.ByBlockIDTransactionID(blockID, txResultErrMsg.TransactionID)
			require.Error(t, err)
			require.ErrorIs(t, err, storage.ErrNotFound)
		}
	})
}

// Test that attempting to batch store transaction result error messages without holding the required lock
// results in an error indicating the missing lock. The implementation should not conflate this error
// case with data for the same key already existing, ie. it should not return [storage.ErrAlreadyExists].
func TestBatchStoreTransactionResultErrorMessagesMissingLock(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		st := store.NewTransactionResultErrorMessages(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txResultErrMsgs := make([]flow.TransactionResultErrorMessage, 0)
		for i := 0; i < 3; i++ {
			expected := flow.TransactionResultErrorMessage{
				TransactionID: unittest.IdentifierFixture(),
				ErrorMessage:  fmt.Sprintf("a runtime error %d", i),
				Index:         uint32(i),
				ExecutorID:    unittest.IdentifierFixture(),
			}
			txResultErrMsgs = append(txResultErrMsgs, expected)
		}

		// Create a context without the required lock
		lockManager := storage.NewTestingLockManager()
		lctx := lockManager.NewContext()
		defer lctx.Release()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return st.BatchStore(lctx, rw, blockID, txResultErrMsgs)
		})
		require.Error(t, err)
		require.NotErrorIs(t, err, storage.ErrAlreadyExists)
		require.Contains(t, err.Error(), "lock_insert_transaction_result_message")
	})
}

// Test that attempting to batch store transaction result error messages while holding the wrong lock
// results in an error indicating the incorrect lock. The implementation should not conflate this error
// case with data for the same key already existing, ie. it should not return [storage.ErrAlreadyExists].
func TestBatchStoreTransactionResultErrorMessagesWrongLock(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		st := store.NewTransactionResultErrorMessages(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txResultErrMsgs := make([]flow.TransactionResultErrorMessage, 0)
		for i := 0; i < 3; i++ {
			expected := flow.TransactionResultErrorMessage{
				TransactionID: unittest.IdentifierFixture(),
				ErrorMessage:  fmt.Sprintf("a runtime error %d", i),
				Index:         uint32(i),
				ExecutorID:    unittest.IdentifierFixture(),
			}
			txResultErrMsgs = append(txResultErrMsgs, expected)
		}

		// Try to use the wrong lock
		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return st.BatchStore(lctx, rw, blockID, txResultErrMsgs)
			})
		})
		require.Error(t, err)
		require.NotErrorIs(t, err, storage.ErrAlreadyExists)
		require.Contains(t, err.Error(), "lock_insert_transaction_result_message")
	})
}
