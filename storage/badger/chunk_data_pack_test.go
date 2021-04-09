package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
)

func TestChunkDataPack(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := badgerstorage.NewChunkDataPacks(&metrics.NoopCollector{}, db, 100)

		// attempt to get an invalid
		_, err := store.ByChunkID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store in db
		chunkID := unittest.IdentifierFixture()
		expected := unittest.ChunkDataPackFixture(chunkID)
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
