package finalizedreader

import (
	"errors"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
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
		all, err := store.InitAll(metrics, db, flow.Emulator)
		require.NoError(t, err)
		blocks := all.Blocks
		headers := all.Headers
		proposal := unittest.ProposalFixture()
		block := proposal.Block

		// store `block`
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, proposal)
			})
		})
		require.NoError(t, err)

		// index `block` as finalized
		err = unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, block.Height, block.ID())
			})
		})
		require.NoError(t, err)

		// verify that `FinalizedReader` reads values from database that are not yet cached, eg. right after initialization
		reader := NewFinalizedReader(headers, block.Height)
		finalized, err := reader.FinalizedBlockIDAtHeight(block.Height)
		require.NoError(t, err)
		require.Equal(t, block.ID(), finalized)

		// verify that `FinalizedReader` returns storage.NotFound when the height is not finalized
		_, err = reader.FinalizedBlockIDAtHeight(block.Height + 1)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound), err)

		// store and finalize one more block
		block2 := unittest.BlockWithParentFixture(block.ToHeader())
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, unittest.ProposalFromBlock(block2))
			})
		})
		require.NoError(t, err)
		err = unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, block2.Height, block2.ID())
			})
		})
		require.NoError(t, err)

		// We declare `block2` as via the `FinalizedReader`
		reader.BlockFinalized(block2.ToHeader())
		require.Equal(t, block.ID(), finalized)

		// should be able to retrieve the block
		finalized, err = reader.FinalizedBlockIDAtHeight(block2.Height)
		require.NoError(t, err)
		require.Equal(t, block2.ID(), finalized)

		// should noop and no panic
		reader.BlockProcessable(block.ToHeader(), block2.ParentQC())
	})
}
