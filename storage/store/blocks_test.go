package store_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
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

		lctx := lockManager.NewContext()
		err := lctx.AcquireLock(storage.LockInsertBlock)
		require.NoError(t, err)

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx, rw, prop)
		})
		require.NoError(t, err)
		lctx.Release()

		retrieved, err := blocks.ByID(block.ID())
		require.NoError(t, err)
		require.Equal(t, &block, retrieved)

		// repeated storage of the same block should return
		lctx2 := lockManager.NewContext()
		require.NoError(t, lctx2.AcquireLock(storage.LockInsertBlock))
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx2, rw, prop)
		})
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
		lctx2.Release()

		// verify after a restart, the block stored in the database is the same
		// as the original
		blocksAfterRestart := store.InitAll(cacheMetrics, db).Blocks
		receivedAfterRestart, err := blocksAfterRestart.ByID(block.ID())
		require.NoError(t, err)
		require.Equal(t, &block, receivedAfterRestart)
	})
}
