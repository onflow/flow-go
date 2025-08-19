package procedure

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInsertRetrieveClusterBlock(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		block := unittest.ClusterBlockFixture()

		_, lctx := unittest.LockManagerWithContext(t, storage.LockInsertOrFinalizeClusterBlock)
		require.NoError(t, lctx.AcquireLock(storage.LockInsertBlock))
		defer lctx.Release()
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return InsertClusterBlock(lctx, rw, unittest.ClusterProposalFromBlock(block))
		}))

		var retrieved cluster.Block
		err := RetrieveClusterBlock(db.Reader(), block.ID(), &retrieved)
		require.NoError(t, err)

		require.Equal(t, *block, retrieved)
	})
}

func TestFinalizeClusterBlock(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		parent := unittest.ClusterBlockFixture()

		block := unittest.ClusterBlockFixture(
			unittest.ClusterBlock.WithParent(parent),
		)

		lockManager := storage.NewTestingLockManager()
		lctx := lockManager.NewContext()
		defer lctx.Release()
		require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
		require.NoError(t, lctx.AcquireLock(storage.LockInsertBlock))
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return InsertClusterBlock(lctx, rw, unittest.ClusterProposalFromBlock(block))
		}))

		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexClusterBlockHeight(lctx, rw.Writer(), block.ChainID, parent.Height, parent.ID())
		}))

		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertClusterFinalizedHeight(lctx, rw.Writer(), block.ChainID, parent.Height)
		}))

		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return FinalizeClusterBlock(lctx, rw, block.ID())
		}))

		var boundary uint64
		var err error
		err = operation.RetrieveClusterFinalizedHeight(db.Reader(), block.ChainID, &boundary)
		require.NoError(t, err)
		require.Equal(t, block.Height, boundary)

		var headID flow.Identifier
		err = operation.LookupClusterBlockHeight(db.Reader(), block.ChainID, boundary, &headID)
		require.NoError(t, err)
		require.Equal(t, block.ID(), headID)
	})
}
