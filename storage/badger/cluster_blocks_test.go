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
		chain := unittest.ClusterBlockChainFixture(4)
		parent, blocks := chain[0], chain[1:]

		// add parent and mark its height as the latest finalized block
		lctx := lockManager.NewContext()
		require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexClusterBlockHeight(lctx, rw.Writer(), parent.Header.ChainID, parent.Header.Height, parent.ID())
		})
		require.NoError(t, err)

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertClusterFinalizedHeight(lctx, rw.Writer(), parent.Header.ChainID, parent.Header.Height)
		})
		require.NoError(t, err)
		lctx.Release()

		// store chain of descending blocks
		for _, block := range blocks {
			lctx2 := lockManager.NewContext()
			require.NoError(t, lctx2.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return procedure.InsertClusterBlock(lctx2, rw, &block)
			})
			require.NoError(t, err)

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return procedure.FinalizeClusterBlock(lctx2, rw, block.Header.ID())
			})
			require.NoError(t, err)
			lctx2.Release()
		}

		clusterBlocks := NewClusterBlocks(
			db,
			blocks[0].Header.ChainID,
			NewHeaders(metrics.NewNoopCollector(), db),
			NewClusterPayloads(metrics.NewNoopCollector(), db),
		)

		t.Run("ByHeight", func(t *testing.T) {
			// check if the block can be retrieved by height
			for _, block := range blocks {
				retrievedBlock, err := clusterBlocks.ByHeight(block.Header.Height)
				require.NoError(t, err)
				require.Equal(t, block.ID(), retrievedBlock.ID())
			}
		})

		t.Run("ByID", func(t *testing.T) {
			// check if the block can be retrieved by ID
			for _, block := range blocks {
				retrievedBlock, err := clusterBlocks.ByID(block.ID())
				require.NoError(t, err)
				require.Equal(t, block.ID(), retrievedBlock.ID())
			}
		})
	})
}
