package badger_test

import (
	"errors"
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
)

func TestIndexStoreRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewIndex(metrics, db)

		blockID := unittest.IdentifierFixture()
		expected := unittest.IndexFixture()

		// retreive without store
		_, err := store.ByBlockID(blockID)
		require.True(t, errors.Is(err, storage.ErrNotFound))

		// store index
		err = store.Store(blockID, expected)
		require.NoError(t, err)

		// retreive index
		actual, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

// Test that we can store and retrieve indexes from a compressed database
func TestIndexStoreRetrieveCompaction(t *testing.T) {
	dbDir := unittest.TempDir(t)
	defer func() {
		require.NoError(t, os.RemoveAll(dbDir))
	}()

	// create database without compression
	compressed := badgerstorage.BadgerOptions(dbDir, nil)

	nocompression := compressed.WithCompression(options.None)
	db, err := badger.Open(nocompression)
	require.NoError(t, err)

	metrics := metrics.NewNoopCollector()
	store := badgerstorage.NewIndex(metrics, db)

	blockID := unittest.IdentifierFixture()
	expected := unittest.IndexFixture()

	// retreive without store
	_, err = store.ByBlockID(blockID)
	require.True(t, errors.Is(err, storage.ErrNotFound))

	// store index
	err = store.Store(blockID, expected)
	require.NoError(t, err)

	// retreive index
	actual, err := store.ByBlockID(blockID)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
	require.NoError(t, db.Close())

	// reopen the database with compression
	cdb, err := badger.Open(compressed)
	require.NoError(t, err)

	cstore := badgerstorage.NewIndex(metrics, cdb)

	// retreive index using the compressed database, and
	// ensure that it is able to read the data saved by un-compressed database
	actual, err = cstore.ByBlockID(blockID)
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	// store a different index using the compressed database,
	// ensure that the compressed index can be retrieved.
	blockID2 := unittest.IdentifierFixture()
	expected2 := unittest.IndexFixture()

	err = cstore.Store(blockID2, expected2)
	require.NoError(t, err)

	actual2, err := cstore.ByBlockID(blockID2)
	require.NoError(t, err)
	require.Equal(t, expected2, actual2)
	require.NoError(t, cdb.Close())
}

// Benchmark that we can store and retrieve indexes from a database
func BenchmarkIndexStore(b *testing.B) {
	unittest.RunWithBadgerDB(b, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewIndex(metrics, db)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			blockID := unittest.IdentifierFixture()
			index := unittest.IndexFixture()

			err := store.Store(blockID, index)
			require.NoError(b, err)
		}
	})
}

// BenchmarkIndexRetrieve benchmarks the retrieval of indexes from the database.
func BenchmarkIndexRetrieve(b *testing.B) {
	unittest.RunWithBadgerDB(b, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewIndex(metrics, db)

		blockID := unittest.IdentifierFixture()
		index := unittest.IndexFixture()

		err := store.Store(blockID, index)
		require.NoError(b, err)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := store.ByBlockID(blockID)
			require.NoError(b, err)
		}
	})
}
