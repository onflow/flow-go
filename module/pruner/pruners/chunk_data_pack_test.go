package pruners

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	storagebadger "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestChunkDataPackPruner(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(badgerDB *badger.DB) {
		unittest.RunWithPebbleDB(t, func(pebbleDB *pebble.DB) {
			m := metrics.NewNoopCollector()
			results := storagebadger.NewExecutionResults(m, badgerDB)
			transactions := storagebadger.NewTransactions(m, badgerDB)
			collections := storagebadger.NewCollections(badgerDB, transactions)
			byChunkIDCacheSize := uint(10)
			pdb := pebbleimpl.ToDB(pebbleDB)
			chunks := store.NewChunkDataPacks(m, pdb, collections, byChunkIDCacheSize)

			// store the chunks
			cdp1, result1 := unittest.ChunkDataPacksFixtureAndResult()
			require.NoError(t, results.Store(result1))
			require.NoError(t, chunks.Store(cdp1))

			pruner := NewChunkDataPackPruner(chunks, results)

			// prune the chunks
			require.NoError(t, pdb.WithReaderBatchWriter(func(w storage.ReaderBatchWriter) error {
				return pruner.PruneByBlockID(result1.BlockID, w)
			}))

			// verify they are pruned
			_, err := chunks.ByChunkID(cdp1[0].ChunkID)
			require.True(t, errors.Is(err, storage.ErrNotFound), fmt.Errorf("expected ErrNotFound but got %v", err))

			// prune again should not return error
			require.NoError(t, pdb.WithReaderBatchWriter(func(w storage.ReaderBatchWriter) error {
				return pruner.PruneByBlockID(result1.BlockID, w)
			}))

			// prune non-exist block should not return error
			require.NoError(t, pdb.WithReaderBatchWriter(func(w storage.ReaderBatchWriter) error {
				return pruner.PruneByBlockID(unittest.IdentifierFixture(), w)
			}))
		})
	})
}
