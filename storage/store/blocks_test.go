package store_test

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

func TestBlockStoreAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		cacheMetrics := &metrics.NoopCollector{}
		// verify after storing a block should be able to retrieve it back
		blocks := store.InitAll(cacheMetrics, db).Blocks
		block := unittest.FullBlockFixture()
		prop := unittest.ProposalFromBlock(block)

		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, prop)
			})
		})
		require.NoError(t, err)

		retrieved, err := blocks.ByID(block.ID())
		require.NoError(t, err)
		require.Equal(t, *block, *retrieved)

		// repeated storage of the same block should return
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx2 lockctx.Context) error {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx2, rw, prop)
			})
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
			return nil
		})
		require.NoError(t, err)

		// verify after a restart, the block stored in the database is the same
		// as the original
		blocksAfterRestart := store.InitAll(cacheMetrics, db).Blocks
		receivedAfterRestart, err := blocksAfterRestart.ByID(block.ID())
		require.NoError(t, err)
		require.Equal(t, *block, *receivedAfterRestart)
	})
}

func TestBlockIndexByHeightAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		cacheMetrics := &metrics.NoopCollector{}
		blocks := store.InitAll(cacheMetrics, db).Blocks
		block := unittest.FullBlockFixture()
		prop := unittest.ProposalFromBlock(block)

		// First store the block
		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, prop)
			})
		})
		require.NoError(t, err)

		// Now index the block by height (requires LockFinalizeBlock)
		err = unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, block.Height, block.ID())
			})
		})
		require.NoError(t, err)

		// Verify we can retrieve the block by height
		retrievedByHeight, err := blocks.ByHeight(block.Height)
		require.NoError(t, err)
		require.Equal(t, *block, *retrievedByHeight)

		// Verify we can retrieve the proposal by height
		retrievedProposalByHeight, err := blocks.ProposalByHeight(block.Height)
		require.NoError(t, err)
		require.Equal(t, *prop, *retrievedProposalByHeight)

		// Test that indexing the same height again returns ErrAlreadyExists
		err = unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, block.Height, block.ID())
			})
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
			return nil
		})
		require.NoError(t, err)

		// Test that retrieving by non-existent height returns ErrNotFound
		_, err = blocks.ByHeight(block.Height + 1000)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// Verify after a restart, the block indexed by height is still retrievable
		blocksAfterRestart := store.InitAll(cacheMetrics, db).Blocks
		receivedAfterRestart, err := blocksAfterRestart.ByHeight(block.Height)
		require.NoError(t, err)
		require.Equal(t, *block, *receivedAfterRestart)
	})
}

func TestBlockIndexByViewAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		cacheMetrics := &metrics.NoopCollector{}
		blocks := store.InitAll(cacheMetrics, db).Blocks
		block := unittest.FullBlockFixture()
		prop := unittest.ProposalFromBlock(block)

		// First store the block and index by view
		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				err := blocks.BatchStore(lctx, rw, prop)
				if err != nil {
					return err
				}
				// Now index the block by view (requires LockInsertBlock)
				return operation.IndexCertifiedBlockByView(lctx, rw, block.View, block.ID())
			})
		})
		require.NoError(t, err)

		// Verify we can retrieve the block by view
		retrievedByView, err := blocks.ByView(block.View)
		require.NoError(t, err)
		require.Equal(t, *block, *retrievedByView)

		// Verify we can retrieve the proposal by view
		retrievedProposalByView, err := blocks.ProposalByView(block.View)
		require.NoError(t, err)
		require.Equal(t, *prop, *retrievedProposalByView)

		// Test that indexing the same view again returns ErrAlreadyExists
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexCertifiedBlockByView(lctx, rw, block.View, block.ID())
			})
		})
		require.ErrorIs(t, err, storage.ErrAlreadyExists)

		// Test that retrieving by non-existent view returns ErrNotFound
		_, err = blocks.ByView(block.View + 1000)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// Verify after a restart, the block indexed by view is still retrievable
		blocksAfterRestart := store.InitAll(cacheMetrics, db).Blocks
		receivedAfterRestart, err := blocksAfterRestart.ByView(block.View)
		require.NoError(t, err)
		require.Equal(t, *block, *receivedAfterRestart)
	})
}
