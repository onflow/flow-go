package pruners

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestChunkDataPackPruner(t *testing.T) {

	unittest.RunWithPebbleDB(t, func(pebbleDB *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		m := metrics.NewNoopCollector()
		db := pebbleimpl.ToDB(pebbleDB)
		results := store.NewExecutionResults(m, db)
		transactions := store.NewTransactions(m, db)
		collections := store.NewCollections(db, transactions)
		byChunkIDCacheSize := uint(10)
		chunks := store.NewChunkDataPacks(m, db, collections, byChunkIDCacheSize)

		// store the chunks
		cdp1, result1 := unittest.ChunkDataPacksFixtureAndResult()
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return results.BatchStore(result1, rw)
		})
		require.NoError(t, unittest.WithLock(t, lockManager, storage.LockInsertChunkDataPack, func(lctx lockctx.Context) error {
			return chunks.StoreByChunkID(lctx, cdp1)
		}))

		pruner := NewChunkDataPackPruner(chunks, results)

		// prune the chunks
		require.NoError(t, db.WithReaderBatchWriter(func(w storage.ReaderBatchWriter) error {
			return pruner.PruneByBlockID(result1.BlockID, w)
		}))

		// verify they are pruned
		_, err = chunks.ByChunkID(cdp1[0].ChunkID)
		require.True(t, errors.Is(err, storage.ErrNotFound), fmt.Errorf("expected ErrNotFound but got %v", err))

		// prune again should not return error
		require.NoError(t, db.WithReaderBatchWriter(func(w storage.ReaderBatchWriter) error {
			return pruner.PruneByBlockID(result1.BlockID, w)
		}))

		// prune non-exist block should not return error
		require.NoError(t, db.WithReaderBatchWriter(func(w storage.ReaderBatchWriter) error {
			return pruner.PruneByBlockID(unittest.IdentifierFixture(), w)
		}))
	})
}
