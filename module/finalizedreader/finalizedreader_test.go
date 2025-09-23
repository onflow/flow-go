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
		proposal := unittest.ProposalFixture()
		block := proposal.Block

		// store `block`
		unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, proposal)
			})
		})

		// index `block` as finalized
		unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, block.Height, block.Hash())
			})
		})

		// verify that `FinalizedReader` reads values from database that are not yet cached, eg. right after initialization
		reader := NewFinalizedReader(headers, block.Height)
		finalized, err := reader.FinalizedBlockIDAtHeight(block.Height)
		require.NoError(t, err)
		require.Equal(t, block.Hash(), finalized)

		// verify that `FinalizedReader` returns storage.NotFound when the height is not finalized
		_, err = reader.FinalizedBlockIDAtHeight(block.Height + 1)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound), err)

		// store and finalize one more block
		block2 := unittest.BlockWithParentFixture(block.ToHeader())
		unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, unittest.ProposalFromBlock(block2))
			})
		})
		unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, block2.Height, block2.Hash())
			})
		})

		// We declare `block2` as via the `FinalizedReader`
		reader.BlockFinalized(block2.ToHeader())
		require.Equal(t, block.Hash(), finalized)

		// should be able to retrieve the block
		finalized, err = reader.FinalizedBlockIDAtHeight(block2.Height)
		require.NoError(t, err)
		require.Equal(t, block2.Hash(), finalized)

		// should noop and no panic
		reader.BlockProcessable(block.ToHeader(), block2.ParentQC())
	})
}
