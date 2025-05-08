package store

import (
	"testing"

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
		metrics := metrics.NewNoopCollector()
		store := NewProtocolKVStore(metrics, db, DefaultProtocolKVStoreCacheSize, DefaultProtocolKVStoreByBlockIDCacheSize)

		expected := &flow.PSKeyValueStoreData{
			Version: 2,
			Data:    unittest.RandomBytes(32),
		}
		stateID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()

		// store protocol state and auxiliary info
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			err := store.BatchStore(rw, stateID, expected)
			require.NoError(t, err)
			return store.BatchIndex(rw, blockID, stateID)
		})
		require.NoError(t, err)

		// fetch protocol state
		actual, err := store.ByID(stateID)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// fetch protocol state by block ID
		actualByBlockID, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		assert.Equal(t, expected, actualByBlockID)
	})
}

// TestProtocolKVStore_StoreTx tests that StoreTx returns an error if the KV-store snapshot with the given id is already stored.
func TestProtocolKVStore_StoreTx(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := NewProtocolKVStore(metrics, db, DefaultProtocolKVStoreCacheSize, DefaultProtocolKVStoreByBlockIDCacheSize)

		stateID := unittest.IdentifierFixture()
		expected := &flow.PSKeyValueStoreData{
			Version: 2,
			Data:    unittest.RandomBytes(32),
		}

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return store.BatchStore(rw, stateID, expected)
		})
		require.NoError(t, err)

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return store.BatchStore(rw, stateID, expected)
		})
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
	})
}

// TestProtocolKVStore_IndexTx tests that IndexTx returns an error if a KV store for the given blockID has already been indexed.
func TestProtocolKVStore_IndexTx(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := NewProtocolKVStore(metrics, db, DefaultProtocolKVStoreCacheSize, DefaultProtocolKVStoreByBlockIDCacheSize)

		stateID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return store.BatchIndex(rw, blockID, stateID)
		})
		require.NoError(t, err)

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return store.BatchIndex(rw, blockID, stateID)
		})
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
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
