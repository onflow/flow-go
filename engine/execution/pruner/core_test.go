package pruner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
	unittestMocks "github.com/onflow/flow-go/utils/unittest/mocks"
)

func TestLoopPruneExecutionDataFromRootToLatestSealed(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(bdb *badger.DB) {
		unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
			// create dependencies
			ps := unittestMocks.NewProtocolState()
			blocks, rootResult, rootSeal := unittest.ChainFixture(0)
			genesis := blocks[0]
			require.NoError(t, ps.Bootstrap(genesis, rootResult, rootSeal))

			ctx, cancel := context.WithCancel(context.Background())
			metrics := metrics.NewNoopCollector()
			headers := badgerstorage.NewHeaders(metrics, bdb)
			results := badgerstorage.NewExecutionResults(metrics, bdb)

			transactions := badgerstorage.NewTransactions(metrics, bdb)
			collections := badgerstorage.NewCollections(bdb, transactions)
			chunkDataPacks := store.NewChunkDataPacks(metrics, pebbleimpl.ToDB(pdb), collections, 1000)

			lastSealedHeight := 30
			lastFinalizedHeight := lastSealedHeight + 2 // 2 finalized but unsealed
			// indexed by height
			chunks := make([]*verification.VerifiableChunkData, lastFinalizedHeight+2)
			parentID := genesis.ID()
			require.NoError(t, headers.Store(genesis.Header))
			for i := 1; i <= lastFinalizedHeight; i++ {
				chunk, block := unittest.VerifiableChunkDataFixture(0, func(header *flow.Header) {
					header.Height = uint64(i)
					header.ParentID = parentID
				})
				chunks[i] = chunk // index by height
				require.NoError(t, headers.Store(chunk.Header))
				require.NoError(t, bdb.Update(operation.IndexBlockHeight(chunk.Header.Height, chunk.Header.ID())))
				require.NoError(t, results.Store(chunk.Result))
				require.NoError(t, results.Index(chunk.Result.BlockID, chunk.Result.ID()))
				require.NoError(t, chunkDataPacks.Store([]*flow.ChunkDataPack{chunk.ChunkDataPack}))
				require.NoError(t, collections.Store(chunk.ChunkDataPack.Collection))
				// verify that chunk data pack fixture can be found by the result
				for _, c := range chunk.Result.Chunks {
					chunkID := c.ID()
					require.Equal(t, chunk.ChunkDataPack.ID(), chunkID)
					_, err := chunkDataPacks.ByChunkID(chunkID)
					require.NoError(t, err)
				}
				// verify the result can be found by block
				_, err := results.ByBlockID(chunk.Header.ID())
				require.NoError(t, err)

				// Finalize block
				require.NoError(t, ps.Extend(block))
				require.NoError(t, ps.Finalize(block.ID()))
				parentID = block.ID()
			}

			// last seale and executed is the last sealed
			require.NoError(t, bdb.Update(operation.InsertExecutedBlock(chunks[lastFinalizedHeight].Header.ID())))
			lastSealed := chunks[lastSealedHeight].Header
			require.NoError(t, ps.MakeSeal(lastSealed.ID()))

			// create config
			cfg := PruningConfig{
				Threshold:                 10,
				BatchSize:                 3,
				SleepAfterEachBatchCommit: 1 * time.Millisecond,
				SleepAfterEachIteration:   100 * time.Millisecond,
			}

			// wait long enough for chunks data packs are pruned
			go (func(cancel func()) {
				time.Sleep(1 * time.Second)
				// cancel the context to stop the loop
				cancel()
			})(cancel)

			require.NoError(t, LoopPruneExecutionDataFromRootToLatestSealed(
				unittest.Logger(), metrics, ctx, ps, bdb, headers, chunkDataPacks, results, pdb, cfg,
			))

			// verify the chunk data packs beyond the threshold are pruned
			// if lastSealedHeight is 2, threshold is 1, then block height 1 and 2 will be stored,
			// and we only prune block 1, the last pruned height is 1 (block 2 is not pruned)
			// so the lastPrunedHeight should be calculated as lastSealedHeight (2) - threshold(1)  = 1
			lastPrunedHeight := lastSealedHeight - int(cfg.Threshold) // 90
			for i := 1; i <= lastPrunedHeight; i++ {
				expected := chunks[i]
				_, err := chunkDataPacks.ByChunkID(expected.ChunkDataPack.ID())
				require.Error(t, err, fmt.Errorf("chunk data pack at height %v should be pruned, but not", i))
				require.ErrorIs(t, err, storage.ErrNotFound)
			}

			// verify the chunk data packs within the threshold are not pruned
			for i := lastPrunedHeight + 1; i <= lastFinalizedHeight; i++ {
				expected := chunks[i]
				actual, err := chunkDataPacks.ByChunkID(expected.ChunkDataPack.ID())
				require.NoError(t, err)
				require.Equal(t, expected.ChunkDataPack, actual)
			}
		})
	})
}
