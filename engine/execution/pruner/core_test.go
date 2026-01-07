package pruner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
	unittestMocks "github.com/onflow/flow-go/utils/unittest/mocks"
)

func TestLoopPruneExecutionDataFromRootToLatestSealed(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		// create dependencies
		ps := unittestMocks.NewProtocolState()
		blocks, rootResult, rootSeal := unittest.ChainFixture(0)
		genesis := blocks[0]
		require.NoError(t, ps.Bootstrap(genesis, rootResult, rootSeal))

		db := pebbleimpl.ToDB(pdb)
		ctx, cancel := context.WithCancel(context.Background())
		metrics := metrics.NewNoopCollector()
		all := store.InitAll(metrics, db)
		headers := all.Headers
		blockstore := all.Blocks
		results := all.Results

		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)
		storedChunkDataPacks := store.NewStoredChunkDataPacks(metrics, db, 1000)
		chunkDataPacks := store.NewChunkDataPacks(metrics, db, storedChunkDataPacks, collections, 1000)

		lastSealedHeight := 30
		lastFinalizedHeight := lastSealedHeight + 2 // 2 finalized but unsealed
		// indexed by height
		chunks := make([]*verification.VerifiableChunkData, lastFinalizedHeight+2)
		parentID := genesis.ID()
		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				// By convention, root block has no proposer signature - implementation has to handle this edge case
				return blockstore.BatchStore(lctx, rw, &flow.Proposal{Block: *genesis, ProposerSigData: nil})
			})
		})
		require.NoError(t, err)

		for i := 1; i <= lastFinalizedHeight; i++ {
			chunk, block := unittest.VerifiableChunkDataFixture(0, func(headerBody *flow.HeaderBody) {
				headerBody.Height = uint64(i)
				headerBody.ParentID = parentID
			})
			chunks[i] = chunk // index by height
			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return blockstore.BatchStore(lctx, rw, unittest.ProposalFromBlock(block))
				})
			})
			require.NoError(t, err)
			err = unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexFinalizedBlockByHeight(lctx, rw, chunk.Header.Height, chunk.Header.ID())
				})
			})
			require.NoError(t, err)
			err = unittest.WithLock(t, lockManager, storage.LockIndexExecutionResult, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					err := results.BatchStore(chunk.Result, rw)
					require.NoError(t, err)

					err = results.BatchIndex(lctx, rw, chunk.Result.BlockID, chunk.Result.ID())
					require.NoError(t, err)
					return nil
				})
			})
			require.NoError(t, err)
			require.NoError(t, unittest.WithLock(t, lockManager, storage.LockIndexChunkDataPackByChunkID, func(lctx lockctx.Context) error {
				storeFunc, err := chunkDataPacks.Store([]*flow.ChunkDataPack{chunk.ChunkDataPack})
				if err != nil {
					return err
				}
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return storeFunc(lctx, rw)
				})
			}))
			_, storeErr := collections.Store(chunk.ChunkDataPack.Collection)
			require.NoError(t, storeErr)
			// verify that chunk data pack fixture can be found by the result
			for _, c := range chunk.Result.Chunks {
				chunkID := c.ID()
				require.Equal(t, chunk.ChunkDataPack.ChunkID, chunkID)
				_, err := chunkDataPacks.ByChunkID(chunkID)
				require.NoError(t, err)
			}
			// verify the result can be found by block
			_, err = results.ByBlockID(chunk.Header.ID())
			require.NoError(t, err)

			// Finalize block
			require.NoError(t, ps.Extend(block))
			require.NoError(t, ps.Finalize(block.ID()))
			parentID = block.ID()
		}

		// update the index "latest executed block (max height)" to latest sealed block
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpdateExecutedBlock(rw.Writer(), chunks[lastFinalizedHeight].Header.ID())
		}))

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
			ctx, unittest.Logger(), metrics, ps, db, headers, chunkDataPacks, results, pdb, cfg,
		))

		// verify the chunk data packs beyond the threshold are pruned
		// if lastSealedHeight is 2, threshold is 1, then block height 1 and 2 will be stored,
		// and we only prune block 1, the last pruned height is 1 (block 2 is not pruned)
		// so the lastPrunedHeight should be calculated as lastSealedHeight (2) - threshold(1)  = 1
		lastPrunedHeight := lastSealedHeight - int(cfg.Threshold) // 90
		for i := 1; i <= lastPrunedHeight; i++ {
			expected := chunks[i]
			_, err := chunkDataPacks.ByChunkID(expected.ChunkDataPack.ChunkID)
			require.Error(t, err, fmt.Errorf("chunk data pack at height %v should be pruned, but not", i))
			require.ErrorIs(t, err, storage.ErrNotFound)
		}

		// verify the chunk data packs within the threshold are not pruned
		for i := lastPrunedHeight + 1; i <= lastFinalizedHeight; i++ {
			expected := chunks[i]
			actual, err := chunkDataPacks.ByChunkID(expected.ChunkDataPack.ChunkID)
			require.NoError(t, err)
			require.Equal(t, expected.ChunkDataPack, actual)
		}
	})
}

func TestEstimateBatchProcessing(t *testing.T) {
	tests := []struct {
		name                      string
		start, end                uint64
		batchSize                 uint
		sleepAfterEachBatchCommit time.Duration
		commitDuration            time.Duration
		expectedBatchCount        uint64
		expectedTotalDuration     time.Duration
	}{
		{
			name:                      "Normal case with multiple batches",
			start:                     0,
			end:                       100,
			batchSize:                 10,
			sleepAfterEachBatchCommit: time.Second,
			commitDuration:            500 * time.Millisecond,
			expectedBatchCount:        11,
			expectedTotalDuration:     10*time.Second + 11*500*time.Millisecond,
		},
		{
			name:                      "Single batch",
			start:                     0,
			end:                       5,
			batchSize:                 10,
			sleepAfterEachBatchCommit: time.Second,
			commitDuration:            500 * time.Millisecond,
			expectedBatchCount:        1,
			expectedTotalDuration:     500 * time.Millisecond,
		},
		{
			name:                      "Zero batch size",
			start:                     0,
			end:                       100,
			batchSize:                 0,
			sleepAfterEachBatchCommit: time.Second,
			commitDuration:            500 * time.Millisecond,
			expectedBatchCount:        0,
			expectedTotalDuration:     0,
		},
		{
			name:                      "Start greater than end",
			start:                     100,
			end:                       50,
			batchSize:                 10,
			sleepAfterEachBatchCommit: time.Second,
			commitDuration:            500 * time.Millisecond,
			expectedBatchCount:        0,
			expectedTotalDuration:     0,
		},
		{
			name:                      "Start equal to end",
			start:                     50,
			end:                       50,
			batchSize:                 10,
			sleepAfterEachBatchCommit: time.Second,
			commitDuration:            500 * time.Millisecond,
			expectedBatchCount:        1,
			expectedTotalDuration:     500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchCount, totalDuration := EstimateBatchProcessing(tt.start, tt.end, tt.batchSize, tt.sleepAfterEachBatchCommit, tt.commitDuration)

			if batchCount != tt.expectedBatchCount {
				t.Errorf("expected batchCount %d, got %d", tt.expectedBatchCount, batchCount)
			}
			if totalDuration != tt.expectedTotalDuration {
				t.Errorf("expected totalDuration %v, got %v", tt.expectedTotalDuration, totalDuration)
			}
		})
	}
}
