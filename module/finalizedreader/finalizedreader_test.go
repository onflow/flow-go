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
		block := unittest.BlockFixture()

<<<<<<< HEAD
		// store block 1
		lockManager := storage.NewTestingLockManager()
		withLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, &block)
			})
		})

		// finalize block 1
		withLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, block.Header.Height, block.ID())
			})
		})

		// verify is able to reader the finalized block ID
		reader := NewFinalizedReader(headers, block.Header.Height)

		finalized, err := reader.FinalizedBlockIDAtHeight(block.Header.Height)
=======
		// store header
		err := headers.Store(unittest.ProposalHeaderFromHeader(block.ToHeader()))
		require.NoError(t, err)

		// index the header
		err = db.Update(operation.IndexBlockHeight(block.Height, block.ID()))
		require.NoError(t, err)

		// verify is able to reader the finalized block ID
		reader := NewFinalizedReader(headers, block.Height)
		finalized, err := reader.FinalizedBlockIDAtHeight(block.Height)
>>>>>>> feature/malleability
		require.NoError(t, err)
		require.Equal(t, block.ID(), finalized)

		// verify is able to return storage.NotFound when the height is not finalized
		_, err = reader.FinalizedBlockIDAtHeight(block.Height + 1)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound), err)

		// finalize one more block
<<<<<<< HEAD
		block2 := unittest.BlockWithParentFixture(block.Header)

		withLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, block2)
			})
		})

		withLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, block2.Header.Height, block2.ID())
			})
		})

		reader.BlockFinalized(block2.Header)
=======
		block2 := unittest.BlockWithParentFixture(block.ToHeader())
		require.NoError(t, headers.Store(unittest.ProposalHeaderFromHeader(block2.ToHeader())))
		err = db.Update(operation.IndexBlockHeight(block2.Height, block2.ID()))
		require.NoError(t, err)
		reader.BlockFinalized(block2.ToHeader())
>>>>>>> feature/malleability

		// should be able to retrieve the block
		finalized, err = reader.FinalizedBlockIDAtHeight(block2.Height)
		require.NoError(t, err)
		require.Equal(t, block2.ID(), finalized)

		// should noop and no panic
		reader.BlockProcessable(block.ToHeader(), block2.ParentQC())
	})
}
