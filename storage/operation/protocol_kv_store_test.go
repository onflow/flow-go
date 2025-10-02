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

// TestInsertProtocolKVStore tests if basic store and index operations on ProtocolKVStore work as expected.
func TestInsertProtocolKVStore(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		expected := &flow.PSKeyValueStoreData{
			Version: 2,
			Data:    unittest.RandomBytes(32),
		}

		kvStoreStateID := unittest.IdentifierFixture()
		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertProtocolKVStore(lctx, rw, kvStoreStateID, expected)
			})
		})
		require.NoError(t, err)

		var actual flow.PSKeyValueStoreData
		err = operation.RetrieveProtocolKVStore(db.Reader(), kvStoreStateID, &actual)
		require.NoError(t, err)

		assert.Equal(t, expected, &actual)

		blockID := unittest.IdentifierFixture()
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexProtocolKVStore(lctx, rw, blockID, kvStoreStateID)
			})
		})
		require.NoError(t, err)

		var actualProtocolKVStoreID flow.Identifier
		err = operation.LookupProtocolKVStore(db.Reader(), blockID, &actualProtocolKVStoreID)
		require.NoError(t, err)

		assert.Equal(t, kvStoreStateID, actualProtocolKVStoreID)
	})
}

// TestInsertProtocolKVStore_ErrAlreadyExists tests that InsertProtocolKVStore returns ErrAlreadyExists
// when attempting to insert a protocol KV store that already exists.
func TestInsertProtocolKVStore_ErrAlreadyExists(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		expected := &flow.PSKeyValueStoreData{
			Version: 2,
			Data:    unittest.RandomBytes(32),
		}

		kvStoreStateID := unittest.IdentifierFixture()

		// First insertion should succeed
		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertProtocolKVStore(lctx, rw, kvStoreStateID, expected)
			})
		})
		require.NoError(t, err)

		// Second insertion with same ID should fail with ErrAlreadyExists
		differentData := &flow.PSKeyValueStoreData{
			Version: 3,
			Data:    unittest.RandomBytes(32),
		}
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertProtocolKVStore(lctx, rw, kvStoreStateID, differentData)
			})
		})
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrAlreadyExists)

		// Verify original data is still there and unchanged
		var actual flow.PSKeyValueStoreData
		err = operation.RetrieveProtocolKVStore(db.Reader(), kvStoreStateID, &actual)
		require.NoError(t, err)
		assert.Equal(t, expected, &actual)
	})
}

// TestIndexProtocolKVStore_ErrAlreadyExists tests that IndexProtocolKVStore returns ErrAlreadyExists
// when attempting to index a protocol KV store for a block ID that already has an index.
func TestIndexProtocolKVStore_ErrAlreadyExists(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		expected := &flow.PSKeyValueStoreData{
			Version: 2,
			Data:    unittest.RandomBytes(32),
		}

		kvStoreStateID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()

		// Insert the protocol KV store first
		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertProtocolKVStore(lctx, rw, kvStoreStateID, expected)
			})
		})
		require.NoError(t, err)

		// First indexing should succeed
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexProtocolKVStore(lctx, rw, blockID, kvStoreStateID)
			})
		})
		require.NoError(t, err)

		// Second indexing with same block ID should fail with ErrAlreadyExists
		differentKVStoreID := unittest.IdentifierFixture()
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexProtocolKVStore(lctx, rw, blockID, differentKVStoreID)
			})
		})
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrAlreadyExists)

		// Verify original index is still there and unchanged
		var actualProtocolKVStoreID flow.Identifier
		err = operation.LookupProtocolKVStore(db.Reader(), blockID, &actualProtocolKVStoreID)
		require.NoError(t, err)
		assert.Equal(t, kvStoreStateID, actualProtocolKVStoreID)
	})
}

// TestInsertProtocolKVStore_MissingLock tests that InsertProtocolKVStore requires LockInsertBlock.
func TestInsertProtocolKVStore_MissingLock(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		expected := &flow.PSKeyValueStoreData{
			Version: 2,
			Data:    unittest.RandomBytes(32),
		}

		kvStoreStateID := unittest.IdentifierFixture()

		// Attempt to insert without holding the required lock
		lctx := lockManager.NewContext()
		defer lctx.Release()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertProtocolKVStore(lctx, rw, kvStoreStateID, expected)
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), storage.LockInsertBlock)
	})
}

// TestIndexProtocolKVStore_MissingLock tests that IndexProtocolKVStore requires LockInsertBlock.
func TestIndexProtocolKVStore_MissingLock(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		kvStoreStateID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()

		// Attempt to index without holding the required lock
		lctx := lockManager.NewContext()
		defer lctx.Release()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexProtocolKVStore(lctx, rw, blockID, kvStoreStateID)
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), storage.LockInsertBlock)
	})
}

// TestInsertProtocolKVStore_WrongLock tests that InsertProtocolKVStore fails when holding wrong locks.
func TestInsertProtocolKVStore_WrongLock(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		expected := &flow.PSKeyValueStoreData{
			Version: 2,
			Data:    unittest.RandomBytes(32),
		}

		kvStoreStateID := unittest.IdentifierFixture()

		// Test with LockFinalizeBlock (wrong lock)
		err := unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertProtocolKVStore(lctx, rw, kvStoreStateID, expected)
			})
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), storage.LockInsertBlock)
	})
}

// TestIndexProtocolKVStore_WrongLock tests that IndexProtocolKVStore fails when holding wrong locks.
func TestIndexProtocolKVStore_WrongLock(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		kvStoreStateID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()

		// Test with LockFinalizeBlock (wrong lock)
		err := unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexProtocolKVStore(lctx, rw, blockID, kvStoreStateID)
			})
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), storage.LockInsertBlock)
	})
}
