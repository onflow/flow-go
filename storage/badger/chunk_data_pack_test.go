package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChunkDataPack_Store evaluates correct storage and retrieval of chunk data packs in the storage.
func TestChunkDataPack_Store(t *testing.T) {
	WithChunkDataPacks(t, 100, func(t *testing.T, chunkDataPacks []*flow.ChunkDataPack, chunkDataPackStore *badgerstorage.ChunkDataPacks) {
		for _, chunkDataPack := range chunkDataPacks {
			err := chunkDataPackStore.Store(chunkDataPack)
			require.NoError(t, err)
		}
	})
}

func WithChunkDataPacks(t *testing.T, chunks int, storeFunc func(*testing.T, []*flow.ChunkDataPack, *badgerstorage.ChunkDataPacks)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		transactions := badgerstorage.NewTransactions(&metrics.NoopCollector{}, db)
		collections := badgerstorage.NewCollections(db, transactions)
		store := badgerstorage.NewChunkDataPacks(&metrics.NoopCollector{}, db, collections, 1)

		//// attempt to get an invalid
		//_, err := store.ByChunkID(unittest.IdentifierFixture())
		//assert.True(t, errors.Is(err, storage.ErrNotFound))

		chunkDataPacks := unittest.ChunkDataPacksFixture(chunks)
		for _, chunkDataPack := range chunkDataPacks {
			// stores collection in Collections storage (which ChunkDataPacks store uses internally)
			err := collections.Store(chunkDataPack.Collection)
			require.NoError(t, err)
		}

		// stores chunk data packs in the memory using provided store function.
		storeFunc(t, chunkDataPacks, store)
		//batch := badgerstorage.NewBatch(db)
		//err = store.BatchStore(expected, batch)
		//require.NoError(t, err)

		// stored chunk data packs should be retrieved successfully.
		for _, expected := range chunkDataPacks {
			actual, err := store.ByChunkID(expected.ChunkID)
			require.NoError(t, err)

			assert.Equal(t, expected, actual)
		}

		//// re-insert - should be idempotent
		//err = store.Store(expected)
		//require.NoError(t, err)
	})
}
