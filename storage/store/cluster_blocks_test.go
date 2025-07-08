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

func TestClusterBlocksByHeight(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		chain := unittest.ClusterBlockChainFixture(5)
		parent, blocks := chain[0], chain[1:]

		lockManager := storage.NewTestingLockManager()
		lctx := lockManager.NewContext()
		require.NoError(t, lctx.AcquireLock(storage.LockFinalizeClusterBlock))
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexClusterBlockHeight(lctx, rw.Writer(), parent.Header.ChainID, parent.Header.Height, parent.ID())
		})
		lctx.Release()
		require.NoError(t, err)

		// add parent as boundary
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertClusterFinalizedHeight(rw.Writer(), parent.Header.ChainID, parent.Header.Height)
		})
		require.NoError(t, err)

		// store a chain of blocks
		for _, block := range blocks {
			_, lctx := unittest.LockManagerWithContext(t, storage.LockInsertClusterBlock)
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return procedure.InsertClusterBlock(lctx, rw, &block)
			})
			require.NoError(t, err)
			lctx.Release()

			lctx = lockManager.NewContext()
			require.NoError(t, lctx.AcquireLock(storage.LockFinalizeClusterBlock))
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return procedure.FinalizeClusterBlock(lctx, rw, block.Header.ID())
			})
			lctx.Release()
			require.NoError(t, err)
		}

		clusterBlocks := NewClusterBlocks(
			db,
			blocks[0].Header.ChainID,
			NewHeaders(metrics.NewNoopCollector(), db),
			NewClusterPayloads(metrics.NewNoopCollector(), db),
		)

		// check if the block can be retrieved by height
		for _, block := range blocks {
			retrievedBlock, err := clusterBlocks.ByHeight(block.Header.Height)
			require.NoError(t, err)
			require.Equal(t, block.ID(), retrievedBlock.ID())
		}
	})
}

func TestClusterBlocks(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		chain := unittest.ClusterBlockChainFixture(4)
		parent, blocks := chain[0], chain[1:]

		lctx := lockManager.NewContext()
		require.NoError(t, lctx.AcquireLock(storage.LockFinalizeClusterBlock))
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexClusterBlockHeight(lctx, rw.Writer(), parent.Header.ChainID, parent.Header.Height, parent.ID())
		})
		lctx.Release()
		require.NoError(t, err)

		// add parent as boundary
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertClusterFinalizedHeight(rw.Writer(), parent.Header.ChainID, parent.Header.Height)
		})
		require.NoError(t, err)

		// store a chain of blocks
		for _, block := range blocks {
			_, lctx := unittest.LockManagerWithContext(t, storage.LockInsertClusterBlock)
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return procedure.InsertClusterBlock(lctx, rw, &block)
			})
			require.NoError(t, err)
			lctx.Release()

			lctx = lockManager.NewContext()
			require.NoError(t, lctx.AcquireLock(storage.LockFinalizeClusterBlock))
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return procedure.FinalizeClusterBlock(lctx, rw, block.Header.ID())
			})
			lctx.Release()
			require.NoError(t, err)
		}

		clusterBlocks := NewClusterBlocks(
			db,
			blocks[0].Header.ChainID,
			NewHeaders(metrics.NewNoopCollector(), db),
			NewClusterPayloads(metrics.NewNoopCollector(), db),
		)

		// check if the block can be retrieved by height
		for _, block := range blocks {
			retrievedBlock, err := clusterBlocks.ByHeight(block.Header.Height)
			require.NoError(t, err)
			require.Equal(t, block.ID(), retrievedBlock.ID())
		}
	})
}
