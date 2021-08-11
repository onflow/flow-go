package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunkDataPack(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		transactions := badgerstorage.NewTransactions(&metrics.NoopCollector{}, db)
		collections := badgerstorage.NewCollections(db, transactions)
		store := badgerstorage.NewChunkDataPacks(&metrics.NoopCollector{}, db, collections, 100)

		// attempt to get an invalid
		_, err := store.ByChunkID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store in db
		chunkID := unittest.IdentifierFixture()
		expected := unittest.ChunkDataPackFixture(chunkID)
		collection := expected.Collection

		// stores collection in Collections storage (which ChunkDataPacks store uses internally)
		err = collections.Store(collection)
		require.NoError(t, err)

		err = store.Store(expected)
		require.NoError(t, err)

		// retrieve the transaction by ID
		actual, err := store.ByChunkID(chunkID)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// re-insert - should be idempotent
		err = store.Store(expected)
		require.NoError(t, err)
	})
}
