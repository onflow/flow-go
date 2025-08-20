package store

import (
	"testing"

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

		lctx := lockManager.NewContext()
		require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexClusterBlockHeight(lctx, rw.Writer(), parent.ChainID, parent.Height, parent.ID())
		})
		require.NoError(t, err)

		// add parent as boundary
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertClusterFinalizedHeight(lctx, rw.Writer(), parent.ChainID, parent.Height)
		})
		require.NoError(t, err)
		lctx.Release()

		// store a chain of blocks
		for _, block := range blocks {
			_, lctx := unittest.LockManagerWithContext(t, storage.LockInsertOrFinalizeClusterBlock)
			require.NoError(t, lctx.AcquireLock(storage.LockInsertBlock))
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return procedure.InsertClusterBlock(lctx, rw, unittest.ClusterProposalFromBlock(block))
			})
			require.NoError(t, err)

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return procedure.FinalizeClusterBlock(lctx, rw, block.ID())
			})
			require.NoError(t, err)
			lctx.Release()
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
