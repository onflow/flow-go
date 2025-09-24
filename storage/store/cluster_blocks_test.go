package store

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/procedure"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestClusterBlocks tests inserting and querying a chain of cluster blocks.
func TestClusterBlocks(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		chain := unittest.ClusterBlockFixtures(5)
		parent, blocks := chain[0], chain[1:]

		// add parent and mark its height as the latest finalized block
		err := unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexClusterBlockHeight(lctx, rw.Writer(), parent.ChainID, parent.Height, parent.ID())
			})
			require.NoError(t, err)

			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertClusterFinalizedHeight(lctx, rw.Writer(), parent.ChainID, parent.Height)
			})
		})
		require.NoError(t, err)

		// store chain of descending blocks
		for _, block := range blocks {
			// InsertClusterBlock only needs LockInsertOrFinalizeClusterBlock
			err := unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx2 lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return procedure.InsertClusterBlock(lctx2, rw, unittest.ClusterProposalFromBlock(block))
				})
			})
			require.NoError(t, err)

			// FinalizeClusterBlock only needs LockInsertOrFinalizeClusterBlock
			err = unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx2 lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return procedure.FinalizeClusterBlock(lctx2, rw, block.ID())
				})
			})
			require.NoError(t, err)
		}

		clusterBlocks := NewClusterBlocks(
			db,
			blocks[0].ChainID,
			NewHeaders(metrics.NewNoopCollector(), db),
			NewClusterPayloads(metrics.NewNoopCollector(), db),
		)

		t.Run("ByHeight", func(t *testing.T) {
			// check if the block can be retrieved by height
			for _, block := range blocks {
				retrievedBlock, err := clusterBlocks.ProposalByHeight(block.Height)
				require.NoError(t, err)
				require.Equal(t, block.ID(), retrievedBlock.Block.ID())
			}
		})

		t.Run("ByID", func(t *testing.T) {
			// check if the block can be retrieved by ID
			for _, block := range blocks {
				retrievedBlock, err := clusterBlocks.ProposalByID(block.ID())
				require.NoError(t, err)
				require.Equal(t, block.ID(), retrievedBlock.Block.ID())
			}
		})
	})
}
