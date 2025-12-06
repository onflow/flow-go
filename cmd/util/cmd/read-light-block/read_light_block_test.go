package read

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReadClusterRange(t *testing.T) {

	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		chain := unittest.ClusterBlockFixtures(5)
		parent, blocks := chain[0], chain[1:]

		err := unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
			// add parent as boundary
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexClusterBlockHeight(lctx, rw, parent.ChainID, parent.Height, parent.ID())
			})
			if err != nil {
				return err
			}

			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.BootstrapClusterFinalizedHeight(lctx, rw, parent.ChainID, parent.Height)
			})
		})
		require.NoError(t, err)

		// add blocks
		for _, block := range blocks {
			err = unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
				err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.InsertClusterBlock(lctx, rw, unittest.ClusterProposalFromBlock(block))
				})
				if err != nil {
					return err
				}

				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.FinalizeClusterBlock(lctx, rw, block.ID())
				})
			})
			require.NoError(t, err)
		}

		clusterBlocks := store.NewClusterBlocks(
			db,
			blocks[0].ChainID,
			store.NewHeaders(metrics.NewNoopCollector(), db, blocks[0].ChainID),
			store.NewClusterPayloads(metrics.NewNoopCollector(), db),
		)

		startHeight := blocks[0].Height
		endHeight := startHeight + 10 // if end height is exceeded the last finalized height, only return up to the last finalized
		lights, err := ReadClusterLightBlockByHeightRange(clusterBlocks, startHeight, endHeight)
		require.NoError(t, err)

		for i, light := range lights {
			require.Equal(t, light.ID, blocks[i].ID())
		}

		require.Equal(t, len(blocks), len(lights))
	})
}
