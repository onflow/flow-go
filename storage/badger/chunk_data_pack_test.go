package badger_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChunkDataPacks_Store evaluates correct storage and retrieval of chunk data packs in the storage.
// It also evaluates that re-inserting is idempotent.
func TestChunkDataPacks_Store(t *testing.T) {
	WithChunkDataPacks(t, 100, func(t *testing.T, chunkDataPacks []*flow.ChunkDataPack, chunkDataPackStore *badgerstorage.ChunkDataPacks, _ *badger.DB) {
		wg := sync.WaitGroup{}
		wg.Add(len(chunkDataPacks))
		for _, chunkDataPack := range chunkDataPacks {
			go func(cdp flow.ChunkDataPack) {
				err := chunkDataPackStore.Store(&cdp)
				require.NoError(t, err)

				wg.Done()
			}(*chunkDataPack)
		}

		unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not store chunk data packs on time")

		// re-insert - should be idempotent
		for _, chunkDataPack := range chunkDataPacks {
			err := chunkDataPackStore.Store(chunkDataPack)
			require.NoError(t, err)
		}
	})
}

// TestChunkDataPack_BatchStore evaluates correct batch storage and retrieval of chunk data packs in the storage.
func TestChunkDataPacks_BatchStore(t *testing.T) {
	WithChunkDataPacks(t, 100, func(t *testing.T, chunkDataPacks []*flow.ChunkDataPack, chunkDataPackStore *badgerstorage.ChunkDataPacks, db *badger.DB) {
		batch := badgerstorage.NewBatch(db)

		wg := sync.WaitGroup{}
		wg.Add(len(chunkDataPacks))
		for _, chunkDataPack := range chunkDataPacks {
			go func(cdp flow.ChunkDataPack) {
				err := chunkDataPackStore.BatchStore(&cdp, batch)
				require.NoError(t, err)

				wg.Done()
			}(*chunkDataPack)
		}

		unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not store chunk data packs on time")

		err := batch.Flush()
		require.NoError(t, err)
	})
}

// TestChunkDataPacks_MissingItem evaluates querying a missing item returns a storage.ErrNotFound error.
func TestChunkDataPacks_MissingItem(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		transactions := badgerstorage.NewTransactions(&metrics.NoopCollector{}, db)
		collections := badgerstorage.NewCollections(db, transactions)
		store := badgerstorage.NewChunkDataPacks(&metrics.NoopCollector{}, db, collections, 1)

		// attempt to get an invalid
		_, err := store.ByChunkID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

// WithChunkDataPacks is a test helper that generates specified number of chunk data packs, store them using the storeFunc, and
// then evaluates whether they are successfully retrieved from storage.
func WithChunkDataPacks(t *testing.T, chunks int, storeFunc func(*testing.T, []*flow.ChunkDataPack, *badgerstorage.ChunkDataPacks, *badger.DB)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		transactions := badgerstorage.NewTransactions(&metrics.NoopCollector{}, db)
		collections := badgerstorage.NewCollections(db, transactions)
		// keep the cache size at 1 to make sure that entries are written and read from storage itself.
		store := badgerstorage.NewChunkDataPacks(&metrics.NoopCollector{}, db, collections, 1)

		chunkDataPacks := unittest.ChunkDataPacksFixture(chunks)
		for _, chunkDataPack := range chunkDataPacks {
			// stores collection in Collections storage (which ChunkDataPacks store uses internally)
			err := collections.Store(chunkDataPack.Collection)
			require.NoError(t, err)
		}

		// stores chunk data packs in the memory using provided store function.
		storeFunc(t, chunkDataPacks, store, db)

		// stored chunk data packs should be retrieved successfully.
		for _, expected := range chunkDataPacks {
			actual, err := store.ByChunkID(expected.ChunkID)
			require.NoError(t, err)

			assert.Equal(t, expected, actual)
		}
	})
}
