package finalizedreader

import (
	"errors"
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

func withLock(t *testing.T, manager lockctx.Manager, lockID string, fn func(lctx lockctx.Context) error) {
	t.Helper()
	lctx := manager.NewContext()
	require.NoError(t, lctx.AcquireLock(lockID))
	defer lctx.Release()
	require.NoError(t, fn(lctx))
}

func TestFinalizedReader(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		// prepare the storage.Headers instance
		metrics := metrics.NewNoopCollector()
		all := store.InitAll(metrics, db)
		blocks := all.Blocks
		headers := all.Headers
		proposal := unittest.ProposalFixture()
		block := proposal.Block

		// store block 1
		lockManager := storage.NewTestingLockManager()
		withLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, proposal)
			})
		})

		// finalize block 1
		withLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, proposal.Block.Height, proposal.Block.ID())
			})
		})

		// verify is able to reader the finalized block ID
		reader := NewFinalizedReader(headers, proposal.Block.Height)

		finalized, err := reader.FinalizedBlockIDAtHeight(proposal.Block.Height)
		require.NoError(t, err)
		require.Equal(t, block.ID(), finalized)

		// verify is able to return storage.NotFound when the height is not finalized
		_, err = reader.FinalizedBlockIDAtHeight(block.Height + 1)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound), err)

		// finalize one more block
		proposal2 := unittest.ProposalFromBlock(unittest.BlockWithParentFixture(block.ToHeader()))
		block2 := proposal2.Block

		withLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, proposal2)
			})
		})

		withLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, block2.Height, block2.ID())
			})
		})

		reader.BlockFinalized(block2.ToHeader())

		// should be able to retrieve the block
		finalized, err = reader.FinalizedBlockIDAtHeight(block2.Height)
		require.NoError(t, err)
		require.Equal(t, block2.ID(), finalized)

		// should noop and no panic
		reader.BlockProcessable(block.ToHeader(), block2.ParentQC())
	})
}
