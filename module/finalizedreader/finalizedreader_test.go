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

func TestFinalizedReader(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		// prepare the storage.Headers instance
		metrics := metrics.NewNoopCollector()
		all := store.InitAll(metrics, db)
		blocks := all.Blocks
		headers := all.Headers
		block1 := unittest.BlockFixture()

<<<<<<< HEAD
		// store `block1`
		unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, &block1)
			})
		})

		// finalize `block1`
		unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, block1.Header.Height, block1.ID())
			})
		})

		// verify that `FinalizedReader` reads values from database that are not yet cached, eg. right after initialization
		reader := NewFinalizedReader(headers, block1.Header.Height)
		finalized, err := reader.FinalizedBlockIDAtHeight(block1.Header.Height)
=======
		// store header
		err := headers.Store(unittest.ProposalHeaderFromHeader(block.ToHeader()))
>>>>>>> master
		require.NoError(t, err)
		require.Equal(t, block1.ID(), finalized)

<<<<<<< HEAD
		// verify that `FinalizedReader` returns storage.NotFound when the height is not finalized
		_, err = reader.FinalizedBlockIDAtHeight(block1.Header.Height + 1)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound), err)

		// store and finalize one more block
		block2 := unittest.BlockWithParentFixture(block1.Header)
		unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, block2)
			})
		})
		unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, block2.Header.Height, block2.ID())
			})
		})

		// We declare `block2` as via the `FinalizedReader`
		reader.BlockFinalized(block2.Header)
=======
		// index the header
		err = db.Update(operation.IndexBlockHeight(block.Height, block.ID()))
		require.NoError(t, err)

		// verify is able to reader the finalized block ID
		reader := NewFinalizedReader(headers, block.Height)
		finalized, err := reader.FinalizedBlockIDAtHeight(block.Height)
		require.NoError(t, err)
		require.Equal(t, block.ID(), finalized)

		// verify is able to return storage.NotFound when the height is not finalized
		_, err = reader.FinalizedBlockIDAtHeight(block.Height + 1)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound), err)

		// finalize one more block
		block2 := unittest.BlockWithParentFixture(block.ToHeader())
		require.NoError(t, headers.Store(unittest.ProposalHeaderFromHeader(block2.ToHeader())))
		err = db.Update(operation.IndexBlockHeight(block2.Height, block2.ID()))
		require.NoError(t, err)
		reader.BlockFinalized(block2.ToHeader())
>>>>>>> master

		// should be able to retrieve the block
		finalized, err = reader.FinalizedBlockIDAtHeight(block2.Height)
		require.NoError(t, err)
		require.Equal(t, block2.ID(), finalized)

<<<<<<< HEAD
		// repeated calls should be noop and no panic
		reader.BlockProcessable(block1.Header, block2.Header.ParentQC())
=======
		// should noop and no panic
		reader.BlockProcessable(block.ToHeader(), block2.ParentQC())
>>>>>>> master
	})
}
