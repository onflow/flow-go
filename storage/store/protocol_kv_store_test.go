package store

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// TesKeyValueStoreStorage tests if the KV store is stored, retrieved and indexed correctly
func TestKeyValueStoreStorage(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		store := NewProtocolKVStore(metrics, db, DefaultProtocolKVStoreCacheSize, DefaultProtocolKVStoreByBlockIDCacheSize)

		expected := &flow.PSKeyValueStoreData{
			Version: 2,
			Data:    unittest.RandomBytes(32),
		}
		stateID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()

		// store protocol state and auxiliary info
		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				err := store.BatchStore(lctx, rw, stateID, expected)
				require.NoError(t, err)
				return store.BatchIndex(lctx, rw, blockID, stateID)
			})
		})
		require.NoError(t, err)

		// fetch protocol state by its own ID
		actual, err := store.ByID(stateID)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// fetch protocol state index by the block ID
		actualByBlockID, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		assert.Equal(t, expected, actualByBlockID)
	})
}

// TestProtocolKVStore_StoreTx tests that StoreTx handles storage request correctly, when a snapshot with
// the given id has already been  stored:
//   - if the KV-store snapshot is exactly the same as the one already stored (incl. the version),  `BatchStore` should return without an error
//   - if we request to store a _different_  KV-store snapshot, an `storage.ErrDataMismatch` should be returned.
func TestProtocolKVStore_StoreTx(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		store := NewProtocolKVStore(metrics, db, DefaultProtocolKVStoreCacheSize, DefaultProtocolKVStoreByBlockIDCacheSize)

		stateID := unittest.IdentifierFixture()
		expected := &flow.PSKeyValueStoreData{
			Version: 2,
			Data:    unittest.RandomBytes(32),
		}

		// Store initial data
		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store.BatchStore(lctx, rw, stateID, expected)
			})
		})
		require.NoError(t, err)

		// Store same data again - should not error
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store.BatchStore(lctx, rw, stateID, expected)
			})
		})
		require.NoError(t, err)

		// Attempt to store different data with the same stateID
		dataDifferent := &flow.PSKeyValueStoreData{
			Version: 2,
			Data:    unittest.RandomBytes(32),
		}

		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store.BatchStore(lctx, rw, stateID, dataDifferent)
			})
		})
		require.ErrorIs(t, err, storage.ErrDataMismatch)

		// Attempt to store different version with the same stateID
		versionDifferent := &flow.PSKeyValueStoreData{
			Version: 3,
			Data:    expected.Data,
		}

		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store.BatchStore(lctx, rw, stateID, versionDifferent)
			})
		})
		require.ErrorIs(t, err, storage.ErrDataMismatch)
	})
}

// TestProtocolKVStore_IndexTx tests that IndexTx handles storage request correctly, when a snapshot with
// the given id has already been indexed:
//   - if the KV-store ID is exactly the same as the one already indexed,  `BatchIndex` should return without an error
//   - if we request to index a different ID, an `storage.ErrDataMismatch` should be returned.
func TestProtocolKVStore_IndexTx(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		store := NewProtocolKVStore(metrics, db, DefaultProtocolKVStoreCacheSize, DefaultProtocolKVStoreByBlockIDCacheSize)

		stateID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()

		// Index initial data
		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store.BatchIndex(lctx, rw, blockID, stateID)
			})
		})
		require.NoError(t, err)

		// Index same data again - should not error
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store.BatchIndex(lctx, rw, blockID, stateID)
			})
		})
		require.NoError(t, err)

		// Attempt to index different stateID with same blockID
		differentStateID := unittest.IdentifierFixture()
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store.BatchIndex(lctx, rw, blockID, differentStateID)
			})
		})
		require.ErrorIs(t, err, storage.ErrDataMismatch)
	})
}

// TestProtocolKVStore_ByBlockID tests that ByBlockID returns an error if no snapshot has been indexed for the given block.
func TestProtocolKVStore_ByBlockID(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := NewProtocolKVStore(metrics, db, DefaultProtocolKVStoreCacheSize, DefaultProtocolKVStoreByBlockIDCacheSize)

		blockID := unittest.IdentifierFixture()
		_, err := store.ByBlockID(blockID)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

// TestProtocolKVStore_ByID tests that ByID returns an error if no snapshot with the given Identifier is known.
func TestProtocolKVStore_ByID(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := NewProtocolKVStore(metrics, db, DefaultProtocolKVStoreCacheSize, DefaultProtocolKVStoreByBlockIDCacheSize)

		stateID := unittest.IdentifierFixture()
		_, err := store.ByID(stateID)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}
