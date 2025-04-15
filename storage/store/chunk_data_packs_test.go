package store_test

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v4"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChunkDataPacks_Store evaluates correct storage and retrieval of chunk data packs in the storage.
// It also evaluates that re-inserting is idempotent.
func TestChunkDataPacks_Store(t *testing.T) {
	WithChunkDataPacks(t, 100, func(t *testing.T, chunkDataPacks []*flow.ChunkDataPack, chunkDataPackStore *store.ChunkDataPacks, _ *badger.DB, _ *pebble.DB) {
		require.NoError(t, chunkDataPackStore.Store(chunkDataPacks))
		require.NoError(t, chunkDataPackStore.Store(chunkDataPacks))
	})
}

func TestChunkDataPack_Remove(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(bdb *badger.DB) {
		unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
			// TODO: once transactions and collections are refactored to use the same storage interface,
			// we can use the same storage.DB for both
			transactions := badgerstorage.NewTransactions(&metrics.NoopCollector{}, bdb)
			collections := badgerstorage.NewCollections(bdb, transactions)
			// keep the cache size at 1 to make sure that entries are written and read from storage itself.
			chunkDataPackStore := store.NewChunkDataPacks(&metrics.NoopCollector{}, pebbleimpl.ToDB(pdb), collections, 1)

			chunkDataPacks := unittest.ChunkDataPacksFixture(10)
			for _, chunkDataPack := range chunkDataPacks {
				// store1s collection in Collections storage (which ChunkDataPacks store uses internally)
				err := collections.Store(chunkDataPack.Collection)
				require.NoError(t, err)
			}

			chunkIDs := make([]flow.Identifier, 0, len(chunkDataPacks))
			for _, chunk := range chunkDataPacks {
				chunkIDs = append(chunkIDs, chunk.ID())
			}

			require.NoError(t, chunkDataPackStore.Store(chunkDataPacks))
			require.NoError(t, chunkDataPackStore.Remove(chunkIDs))

			// verify it has been removed
			_, err := chunkDataPackStore.ByChunkID(chunkIDs[0])
			assert.ErrorIs(t, err, storage.ErrNotFound)

			// Removing again should not error
			require.NoError(t, chunkDataPackStore.Remove(chunkIDs))
		})
	})
}

// TestChunkDataPacks_MissingItem evaluates querying a missing item returns a storage.ErrNotFound error.
func TestChunkDataPacks_MissingItem(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(bdb *badger.DB) {
		unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
			// TODO: once transactions and collections are refactored to use the same storage interface,
			// we can use the same storage.DB for both
			transactions := badgerstorage.NewTransactions(&metrics.NoopCollector{}, bdb)
			collections := badgerstorage.NewCollections(bdb, transactions)
			store1 := store.NewChunkDataPacks(&metrics.NoopCollector{}, pebbleimpl.ToDB(pdb), collections, 1)

			// attempt to get an invalid
			_, err := store1.ByChunkID(unittest.IdentifierFixture())
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})
	})
}

// TestChunkDataPacks_StoreTwice evaluates that storing the same chunk data pack twice
// does not result in an error.
func TestChunkDataPacks_StoreTwice(t *testing.T) {
	WithChunkDataPacks(t, 2, func(t *testing.T, chunkDataPacks []*flow.ChunkDataPack, chunkDataPackStore *store.ChunkDataPacks, bdb *badger.DB, pdb *pebble.DB) {
		transactions := badgerstorage.NewTransactions(&metrics.NoopCollector{}, bdb)
		collections := badgerstorage.NewCollections(bdb, transactions)
		store1 := store.NewChunkDataPacks(&metrics.NoopCollector{}, pebbleimpl.ToDB(pdb), collections, 1)
		require.NoError(t, store1.Store(chunkDataPacks))

		for _, c := range chunkDataPacks {
			c2, err := store1.ByChunkID(c.ChunkID)
			require.NoError(t, err)
			require.Equal(t, c, c2)
		}

		require.NoError(t, store1.Store(chunkDataPacks))
	})
}

// WithChunkDataPacks is a test helper that generates specified number of chunk data packs, store1 them using the storeFunc, and
// then evaluates whether they are successfully retrieved from storage.
func WithChunkDataPacks(t *testing.T, chunks int, storeFunc func(*testing.T, []*flow.ChunkDataPack, *store.ChunkDataPacks, *badger.DB, *pebble.DB)) {
	unittest.RunWithBadgerDB(t, func(bdb *badger.DB) {
		unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
			// TODO: once transactions and collections are refactored to use the same storage interface,
			// we can use the same storage.DB for both
			transactions := badgerstorage.NewTransactions(&metrics.NoopCollector{}, bdb)
			collections := badgerstorage.NewCollections(bdb, transactions)
			// keep the cache size at 1 to make sure that entries are written and read from storage itself.
			store1 := store.NewChunkDataPacks(&metrics.NoopCollector{}, pebbleimpl.ToDB(pdb), collections, 1)

			chunkDataPacks := unittest.ChunkDataPacksFixture(chunks)
			for _, chunkDataPack := range chunkDataPacks {
				// store1s collection in Collections storage (which ChunkDataPacks store uses internally)
				err := collections.Store(chunkDataPack.Collection)
				require.NoError(t, err)
			}

			// store1s chunk data packs in the memory using provided store function.
			storeFunc(t, chunkDataPacks, store1, bdb, pdb)

			// store1d chunk data packs should be retrieved successfully.
			for _, expected := range chunkDataPacks {
				actual, err := store1.ByChunkID(expected.ChunkID)
				require.NoError(t, err)

				assert.Equal(t, expected, actual)
			}
		})
	})
}
