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
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertProtocolKVStore(rw, kvStoreStateID, expected)
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
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertProtocolKVStore(rw, kvStoreStateID, expected)
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
