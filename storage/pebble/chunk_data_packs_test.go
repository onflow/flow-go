package pebble

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMsgPacks(t *testing.T) {
	chunkDataPacks := unittest.ChunkDataPacksFixture(10)
	for _, chunkDataPack := range chunkDataPacks {
		sc := storage.ToStoredChunkDataPack(chunkDataPack)
		value, err := msgpack.Marshal(sc)
		require.NoError(t, err)

		var actual storage.StoredChunkDataPack
		err = msgpack.Unmarshal(value, &actual)
		require.NoError(t, err)

		require.Equal(t, *sc, actual)
	}
}

// TestChunkDataPacks_Store evaluates correct storage and retrieval of chunk data packs in the storage.
// It also evaluates that re-inserting is idempotent.
func TestChunkDataPacks_Store(t *testing.T) {
	WithChunkDataPacks(t, 100, func(t *testing.T, chunkDataPacks []*flow.ChunkDataPack, chunkDataPackStore storage.ChunkDataPacks, _ *pebble.DB) {
		// can store
		require.NoError(t, chunkDataPackStore.Store(chunkDataPacks))

		// can read back
		for _, c := range chunkDataPacks {
			c2, err := chunkDataPackStore.ByChunkID(c.ChunkID)
			require.NoError(t, err)
			require.Equal(t, c, c2)
		}

		// can store again
		require.NoError(t, chunkDataPackStore.Store(chunkDataPacks))

		cids := make([]flow.Identifier, 0, len(chunkDataPacks))
		for i, c := range chunkDataPacks {
			// remove everything except the first one
			if i > 0 {
				cids = append(cids, c.ChunkID)
			}
		}
		// can remove
		require.NoError(t, chunkDataPackStore.Remove(cids))
		for i, c := range chunkDataPacks {
			if i == 0 {
				// the first one is not removed
				_, err := chunkDataPackStore.ByChunkID(c.ChunkID)
				require.NoError(t, err)
				continue
			}
			// the rest are removed
			_, err := chunkDataPackStore.ByChunkID(c.ChunkID)
			require.True(t, errors.Is(err, storage.ErrNotFound))
		}

		// can remove again
		require.NoError(t, chunkDataPackStore.Remove(cids))
	})
}

// WithChunkDataPacks is a test helper that generates specified number of chunk data packs, store them using the storeFunc, and
// then evaluates whether they are successfully retrieved from storage.
func WithChunkDataPacks(t *testing.T, chunks int, storeFunc func(*testing.T, []*flow.ChunkDataPack, storage.ChunkDataPacks, *pebble.DB)) {
	RunWithBadgerDBAndPebbleDB(t, func(badgerDB *badger.DB, db *pebble.DB) {
		transactions := NewTransactions(&metrics.NoopCollector{}, db)
		collections := NewCollections(db, transactions)
		// keep the cache size at 1 to make sure that entries are written and read from storage itself.
		store := NewChunkDataPacks(&metrics.NoopCollector{}, db, collections, 1)

		chunkDataPacks := unittest.ChunkDataPacksFixture(chunks)
		for _, chunkDataPack := range chunkDataPacks {
			// stores collection in Collections storage (which ChunkDataPacks store uses internally)
			err := collections.Store(chunkDataPack.Collection)
			require.NoError(t, err)
		}

		// stores chunk data packs in the memory using provided store function.
		storeFunc(t, chunkDataPacks, store, db)
	})
}

func RunWithBadgerDBAndPebbleDB(t *testing.T, fn func(*badger.DB, *pebble.DB)) {
	unittest.RunWithTempDir(t, func(dir string) {
		badgerDB := unittest.BadgerDB(t, filepath.Join(dir, "badger"))
		defer func() {
			require.NoError(t, badgerDB.Close())
		}()

		cache := pebble.NewCache(1 << 20)
		defer cache.Unref()
		// currently pebble is only used for registers
		opts := DefaultPebbleOptions(cache, pebble.DefaultComparer)
		pebbledb, err := pebble.Open(filepath.Join(dir, "pebble"), opts)
		require.NoError(t, err)
		defer pebbledb.Close()

		fn(badgerDB, pebbledb)
	})
}
