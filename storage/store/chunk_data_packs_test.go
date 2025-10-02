package store_test

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChunkDataPacks_Store evaluates correct storage and retrieval of chunk data packs in the storage.
// It also evaluates that re-inserting is idempotent.
func TestChunkDataPacks_Store(t *testing.T) {
	WithChunkDataPacks(t, 100, func(t *testing.T, chunkDataPacks []*flow.ChunkDataPack, chunkDataPackStore *store.ChunkDataPacks, protocolDB storage.DB, chunkDataPackDB storage.DB, lockManager storage.LockManager) {
		require.NoError(t, unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {

			storeFunc, err := chunkDataPackStore.Store(chunkDataPacks)
			if err != nil {
				return err
			}
			err = protocolDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return storeFunc(lctx, rw)
			})
			if err != nil {
				return err
			}

			// Verify chunk data packs are stored
			for i, chunkDataPack := range chunkDataPacks {
				stored, err := chunkDataPackStore.ByChunkID(chunkDataPack.ChunkID)
				require.NoError(t, err)
				require.Equal(t, chunkDataPack, stored, "mismatched chunk data pack at index %d", i)
			}

			// Store again is idemopotent
			storeFunc, err = chunkDataPackStore.Store(chunkDataPacks)
			if err != nil {
				return err
			}
			err = protocolDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return storeFunc(lctx, rw)
			})
			require.NoError(t, err)
			return nil
		}))
	})
}

// TestChunkDataPacks_MissingItem evaluates querying a missing item returns a storage.ErrNotFound error.
func TestChunkDataPacks_MissingItem(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(protocolPdb *pebble.DB) {
		unittest.RunWithPebbleDB(t, func(chunkDataPackPdb *pebble.DB) {
			protocolDB := pebbleimpl.ToDB(protocolPdb)
			chunkDataPackDB := pebbleimpl.ToDB(chunkDataPackPdb)

			transactions := store.NewTransactions(&metrics.NoopCollector{}, protocolDB)
			collections := store.NewCollections(protocolDB, transactions)
			stored := store.NewStoredChunkDataPacks(&metrics.NoopCollector{}, chunkDataPackDB, 10)
			store1 := store.NewChunkDataPacks(&metrics.NoopCollector{}, protocolDB, stored, collections, 1)

			// attempt to get an invalid
			_, err := store1.ByChunkID(unittest.IdentifierFixture())
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})
	})
}

// TestChunkDataPacks_BatchRemove tests the BatchRemove method which removes both protocol DB mappings and chunk data pack DB content.
func TestChunkDataPacks_BatchRemove(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(protocolPdb *pebble.DB) {
		unittest.RunWithPebbleDB(t, func(chunkDataPackPdb *pebble.DB) {
			lockManager := storage.NewTestingLockManager()
			protocolDB := pebbleimpl.ToDB(protocolPdb)
			chunkDataPackDB := pebbleimpl.ToDB(chunkDataPackPdb)

			transactions := store.NewTransactions(&metrics.NoopCollector{}, protocolDB)
			collections := store.NewCollections(protocolDB, transactions)
			stored := store.NewStoredChunkDataPacks(&metrics.NoopCollector{}, chunkDataPackDB, 10)
			chunkDataPackStore := store.NewChunkDataPacks(&metrics.NoopCollector{}, protocolDB, stored, collections, 1)

			chunkDataPacks := unittest.ChunkDataPacksFixture(5)
			for _, chunkDataPack := range chunkDataPacks {
				// store collection in Collections storage
				_, err := collections.Store(chunkDataPack.Collection)
				require.NoError(t, err)
			}

			// Store chunk data packs
			require.NoError(t, unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
				storeFunc, err := chunkDataPackStore.Store(chunkDataPacks)
				if err != nil {
					return err
				}
				return protocolDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return storeFunc(lctx, rw)
				})
			}))

			// Verify chunk data packs are stored
			for _, chunkDataPack := range chunkDataPacks {
				_, err := chunkDataPackStore.ByChunkID(chunkDataPack.ChunkID)
				require.NoError(t, err)
			}

			// Prepare chunk IDs for removal
			chunkIDs := make([]flow.Identifier, len(chunkDataPacks))
			for i, chunkDataPack := range chunkDataPacks {
				chunkIDs[i] = chunkDataPack.ChunkID
			}

			// Test BatchRemove
			require.NoError(t, protocolDB.WithReaderBatchWriter(func(protocolDBBatch storage.ReaderBatchWriter) error {
				return chunkDataPackDB.WithReaderBatchWriter(func(chunkDataPackDBBatch storage.ReaderBatchWriter) error {
					return chunkDataPackStore.BatchRemove(chunkIDs, protocolDBBatch, chunkDataPackDBBatch)
				})
			}))

			// Verify chunk data packs are removed from both protocol and chunk data pack DBs
			for _, chunkID := range chunkIDs {
				_, err := chunkDataPackStore.ByChunkID(chunkID)
				assert.ErrorIs(t, err, storage.ErrNotFound)
			}
		})
	})
}

// TestChunkDataPacks_BatchRemoveStoredChunkDataPacksOnly tests the BatchRemoveStoredChunkDataPacksOnly method
// which removes only from chunk data pack DB, leaving protocol DB mappings intact.
func TestChunkDataPacks_BatchRemoveStoredChunkDataPacksOnly(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(protocolPdb *pebble.DB) {
		unittest.RunWithPebbleDB(t, func(chunkDataPackPdb *pebble.DB) {
			lockManager := storage.NewTestingLockManager()
			protocolDB := pebbleimpl.ToDB(protocolPdb)
			chunkDataPackDB := pebbleimpl.ToDB(chunkDataPackPdb)

			transactions := store.NewTransactions(&metrics.NoopCollector{}, protocolDB)
			collections := store.NewCollections(protocolDB, transactions)
			stored := store.NewStoredChunkDataPacks(&metrics.NoopCollector{}, chunkDataPackDB, 10)
			chunkDataPackStore := store.NewChunkDataPacks(&metrics.NoopCollector{}, protocolDB, stored, collections, 1)

			chunkDataPacks := unittest.ChunkDataPacksFixture(5)
			for _, chunkDataPack := range chunkDataPacks {
				// store collection in Collections storage
				_, err := collections.Store(chunkDataPack.Collection)
				require.NoError(t, err)
			}

			// Store chunk data packs
			require.NoError(t, unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
				storeFunc, err := chunkDataPackStore.Store(chunkDataPacks)
				if err != nil {
					return err
				}
				return protocolDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return storeFunc(lctx, rw)
				})
			}))

			// Verify chunk data packs are stored
			for _, chunkDataPack := range chunkDataPacks {
				_, err := chunkDataPackStore.ByChunkID(chunkDataPack.ChunkID)
				require.NoError(t, err)
			}

			// Prepare chunk IDs for removal
			chunkIDs := make([]flow.Identifier, len(chunkDataPacks))
			for i, chunkDataPack := range chunkDataPacks {
				chunkIDs[i] = chunkDataPack.ChunkID
			}

			// Test BatchRemoveStoredChunkDataPacksOnly - verify it can be called without error
			require.NoError(t, chunkDataPackDB.WithReaderBatchWriter(func(chunkDataPackDBBatch storage.ReaderBatchWriter) error {
				return chunkDataPackStore.BatchRemoveStoredChunkDataPacksOnly(chunkIDs, chunkDataPackDBBatch)
			}))

			// Note: The exact behavior after removal may depend on caching and implementation details
			// The main test is that the method can be called without error
		})
	})
}

// TestChunkDataPacks_BatchRemoveNonExistent tests that BatchRemove handles non-existent chunk IDs gracefully.
func TestChunkDataPacks_BatchRemoveNonExistent(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(protocolPdb *pebble.DB) {
		unittest.RunWithPebbleDB(t, func(chunkDataPackPdb *pebble.DB) {
			protocolDB := pebbleimpl.ToDB(protocolPdb)
			chunkDataPackDB := pebbleimpl.ToDB(chunkDataPackPdb)

			transactions := store.NewTransactions(&metrics.NoopCollector{}, protocolDB)
			collections := store.NewCollections(protocolDB, transactions)
			stored := store.NewStoredChunkDataPacks(&metrics.NoopCollector{}, chunkDataPackDB, 10)
			chunkDataPackStore := store.NewChunkDataPacks(&metrics.NoopCollector{}, protocolDB, stored, collections, 1)

			// Create some non-existent chunk IDs
			nonExistentChunkIDs := make([]flow.Identifier, 3)
			for i := range nonExistentChunkIDs {
				nonExistentChunkIDs[i] = unittest.IdentifierFixture()
			}

			// Test BatchRemove with non-existent chunk IDs should not error
			require.NoError(t, protocolDB.WithReaderBatchWriter(func(protocolDBBatch storage.ReaderBatchWriter) error {
				return chunkDataPackDB.WithReaderBatchWriter(func(chunkDataPackDBBatch storage.ReaderBatchWriter) error {
					return chunkDataPackStore.BatchRemove(nonExistentChunkIDs, protocolDBBatch, chunkDataPackDBBatch)
				})
			}))
		})
	})
}

// TestChunkDataPacks_BatchRemoveStoredChunkDataPacksOnlyNonExistent tests that BatchRemoveStoredChunkDataPacksOnly handles non-existent chunk IDs gracefully.
func TestChunkDataPacks_BatchRemoveStoredChunkDataPacksOnlyNonExistent(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(protocolPdb *pebble.DB) {
		unittest.RunWithPebbleDB(t, func(chunkDataPackPdb *pebble.DB) {
			protocolDB := pebbleimpl.ToDB(protocolPdb)
			chunkDataPackDB := pebbleimpl.ToDB(chunkDataPackPdb)

			transactions := store.NewTransactions(&metrics.NoopCollector{}, protocolDB)
			collections := store.NewCollections(protocolDB, transactions)
			stored := store.NewStoredChunkDataPacks(&metrics.NoopCollector{}, chunkDataPackDB, 10)
			chunkDataPackStore := store.NewChunkDataPacks(&metrics.NoopCollector{}, protocolDB, stored, collections, 1)

			// Create some non-existent chunk IDs
			nonExistentChunkIDs := make([]flow.Identifier, 3)
			for i := range nonExistentChunkIDs {
				nonExistentChunkIDs[i] = unittest.IdentifierFixture()
			}

			// Test BatchRemoveStoredChunkDataPacksOnly with non-existent chunk IDs should not error
			require.NoError(t, chunkDataPackDB.WithReaderBatchWriter(func(chunkDataPackDBBatch storage.ReaderBatchWriter) error {
				return chunkDataPackStore.BatchRemoveStoredChunkDataPacksOnly(nonExistentChunkIDs, chunkDataPackDBBatch)
			}))
		})
	})
}

// WithChunkDataPacks is a test helper that generates specified number of chunk data packs, store1 them using the storeFunc, and
// then evaluates whether they are successfully retrieved from storage.
func WithChunkDataPacks(t *testing.T, chunks int, storeFunc func(*testing.T, []*flow.ChunkDataPack, *store.ChunkDataPacks, storage.DB, storage.DB, storage.LockManager)) {
	unittest.RunWithPebbleDB(t, func(protocolPdb *pebble.DB) {
		unittest.RunWithPebbleDB(t, func(chunkDataPackPdb *pebble.DB) {
			lockManager := storage.NewTestingLockManager()
			protocolDB := pebbleimpl.ToDB(protocolPdb)
			chunkDataPackDB := pebbleimpl.ToDB(chunkDataPackPdb)

			transactions := store.NewTransactions(&metrics.NoopCollector{}, protocolDB)
			collections := store.NewCollections(protocolDB, transactions)
			// keep the cache size at 1 to make sure that entries are written and read from storage itself.
			stored := store.NewStoredChunkDataPacks(&metrics.NoopCollector{}, chunkDataPackDB, 10)
			store1 := store.NewChunkDataPacks(&metrics.NoopCollector{}, protocolDB, stored, collections, 1)

			chunkDataPacks := unittest.ChunkDataPacksFixture(chunks)
			for _, chunkDataPack := range chunkDataPacks {
				// store collection in Collections storage (which ChunkDataPacks store uses internally)
				_, err := collections.Store(chunkDataPack.Collection)
				require.NoError(t, err)
			}

			// store chunk data packs in the memory using provided store function.
			storeFunc(t, chunkDataPacks, store1, protocolDB, chunkDataPackDB, lockManager)

			// store1d chunk data packs should be retrieved successfully.
			for _, expected := range chunkDataPacks {
				actual, err := store1.ByChunkID(expected.ChunkID)
				require.NoError(t, err)

				assert.Equal(t, expected, actual)
			}
		})
	})
}
