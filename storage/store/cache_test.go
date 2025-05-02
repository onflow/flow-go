package store

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestCache_Exists tests existence checking items in the cache.
func TestCache_Exists(t *testing.T) {
	cache := newCache[flow.Identifier, any](metrics.NewNoopCollector(), "test")

	t.Run("non-existent", func(t *testing.T) {
		key := unittest.IdentifierFixture()
		exists := cache.IsCached(key)
		assert.False(t, exists)
	})

	t.Run("existent", func(t *testing.T) {
		key := unittest.IdentifierFixture()
		cache.Insert(key, unittest.RandomBytes(128))

		exists := cache.IsCached(key)
		assert.True(t, exists)
	})

	t.Run("removed", func(t *testing.T) {
		key := unittest.IdentifierFixture()
		// insert, then remove the item
		cache.Insert(key, unittest.RandomBytes(128))
		cache.Remove(key)

		exists := cache.IsCached(key)
		assert.False(t, exists)
	})
}

// Test storing an item will be cached, and when cache hit,
// the retrieve function is only called once
func TestCache_CachedHit(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		retrieved := atomic.NewUint64(0)

		store := func(rw storage.ReaderBatchWriter, key flow.Identifier, val []byte) error {
			return operation.UpsertByKey(rw.Writer(), key[:], val)
		}
		retrieve := func(r storage.Reader, key flow.Identifier) ([]byte, error) {
			retrieved.Inc()
			var val []byte
			err := operation.RetrieveByKey(r, key[:], &val)
			if err != nil {
				return nil, err
			}
			return val, nil
		}

		cache := newCache(metrics.NewNoopCollector(), "test",
			withStore(store),
			withRetrieve(retrieve),
		)

		key := unittest.IdentifierFixture()
		val := unittest.RandomBytes(128)

		// storing the item will cache it
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return cache.PutTx(rw, key, val)
		}))

		// retrieving stored item should hit the cache, no db op is called
		cached, err := cache.Get(db.Reader(), key)
		require.NoError(t, err)
		require.Equal(t, val, cached)
		require.Equal(t, uint64(0), retrieved.Load()) // no db op

		// removing the cached item
		cache.Remove(key)

		// Get the same item, the cached item will miss, and retrieve from db, so db op is called
		cached, err = cache.Get(db.Reader(), key)
		require.NoError(t, err)
		require.Equal(t, val, cached)
		require.Equal(t, uint64(1), retrieved.Load()) // hit db

		// Get the same item again, hit cache
		_, err = cache.Get(db.Reader(), key)
		require.NoError(t, err)
		require.Equal(t, uint64(1), retrieved.Load()) // cache hit

		// Query other key will hit db
		_, err = cache.Get(db.Reader(), unittest.IdentifierFixture())
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

// Test storage.ErrNotFound is returned when cache is missing
// and is not cached
func TestCache_NotFoundReturned(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		retrieved := atomic.NewUint64(0)
		retrieve := func(r storage.Reader, key flow.Identifier) ([]byte, error) {
			retrieved.Inc()
			return nil, storage.ErrNotFound
		}

		cache := newCache(metrics.NewNoopCollector(), "test",
			withRetrieve(retrieve),
		)

		// Create a random identifier to use as a key
		notExist := unittest.IdentifierFixture()

		// Try to get the non-existent item from the cache
		// Assert that the error is storage.ErrNotFound
		_, err := cache.Get(db.Reader(), notExist)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// Get the item again, this time the cache should not be used
		_, err = cache.Get(db.Reader(), notExist)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.Equal(t, uint64(2), retrieved.Load()) // retrieved from DB 2 times.
	})
}

// Test when store return exception error, the key value is not cached,
func TestCache_ExceptionNotCached(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		storeException := fmt.Errorf("storing exception")
		stored, retrieved := atomic.NewUint64(0), atomic.NewUint64(0)

		store := func(rw storage.ReaderBatchWriter, key flow.Identifier, val []byte) error {
			stored.Inc()
			return storeException
		}
		retrieve := func(r storage.Reader, key flow.Identifier) ([]byte, error) {
			retrieved.Inc()
			var val []byte
			err := operation.RetrieveByKey(r, key[:], &val)
			if err != nil {
				return nil, err
			}
			return val, nil
		}

		cache := newCache(metrics.NewNoopCollector(), "test",
			withStore(store),
			withRetrieve(retrieve),
		)

		key := unittest.IdentifierFixture()
		val := unittest.RandomBytes(128)

		// store returns exception err
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return cache.PutTx(rw, key, val)
		})

		require.ErrorIs(t, err, storeException)

		// assert key value is not cached
		_, err = cache.Get(db.Reader(), key)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}
