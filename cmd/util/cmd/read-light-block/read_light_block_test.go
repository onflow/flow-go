package read

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/procedure"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReadClusterRange(t *testing.T) {

	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		chain := unittest.ClusterBlockChainFixture(5)
		parent, blocks := chain[0], chain[1:]

		lockManager := storage.NewTestingLockManager()

		lctx := lockManager.NewContext()
		require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
		// add parent as boundary
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexClusterBlockHeight(lctx, rw.Writer(), parent.Header.ChainID, parent.Header.Height, parent.ID())
		})
		require.NoError(t, err)
		lctx.Release()

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertClusterFinalizedHeight(rw.Writer(), parent.Header.ChainID, parent.Header.Height)
		})
		require.NoError(t, err)

		// add blocks
		for _, block := range blocks {
			lctx := lockManager.NewContext()
			require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return procedure.InsertClusterBlock(lctx, rw, &block)
			})
			lctx.Release()
			require.NoError(t, err)

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				lctx := lockManager.NewContext()
				defer lctx.Release()
				if err := lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock); err != nil {
					return err
				}
				return procedure.FinalizeClusterBlock(lctx, rw, block.Header.ID())
			})
			require.NoError(t, err)
		}

		clusterBlocks := store.NewClusterBlocks(
			db,
			blocks[0].Header.ChainID,
			store.NewHeaders(metrics.NewNoopCollector(), db),
			store.NewClusterPayloads(metrics.NewNoopCollector(), db),
		)

		startHeight := blocks[0].Header.Height
		endHeight := startHeight + 10 // if end height is exceeded the last finalized height, only return up to the last finalized
		lights, err := ReadClusterLightBlockByHeightRange(clusterBlocks, startHeight, endHeight)
		require.NoError(t, err)

		for i, light := range lights {
			require.Equal(t, light.ID, blocks[i].ID())
		}

		require.Equal(t, len(blocks), len(lights))
	})
}
