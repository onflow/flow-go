package procedure

import (
	"testing"

	"github.com/jordanschalm/lockctx"
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
			return InsertClusterBlock(lctx, rw, &block)
		}))

		var retrieved cluster.Block
		err := RetrieveClusterBlock(db.Reader(), block.ID(), &retrieved)
		require.NoError(t, err)

		require.Equal(t, block, retrieved)
	})
}

func TestFinalizeClusterBlock(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		parent := unittest.ClusterBlockFixture()
		block := unittest.ClusterBlockWithParent(&parent)

		lockManager := storage.NewTestingLockManager()
		lctx := lockManager.NewContext()
		defer lctx.Release()
		require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
		require.NoError(t, lctx.AcquireLock(storage.LockInsertBlock))
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return InsertClusterBlock(lctx, rw, &parent)
		}))

		// index parent as latest finalized block (manually writing respective indexes like in bootstrapping to skip transitive consistency checks)
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexClusterBlockHeight(lctx, rw.Writer(), block.Header.ChainID, parent.Header.Height, parent.ID())
		}))
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertClusterFinalizedHeight(lctx, rw.Writer(), block.Header.ChainID, parent.Header.Height)
		}))

		// Insert new block and verify `FinalizeClusterBlock` procedure accepts it
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return InsertClusterBlock(lctx, rw, &block)
		}))
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return FinalizeClusterBlock(lctx, rw, block.Header.ID())
		}))

		// verify that the new block as been properly indexed as the latest finalized
		var latestFinalizedHeight uint64
		var err error
		err = operation.RetrieveClusterFinalizedHeight(db.Reader(), block.Header.ChainID, &latestFinalizedHeight)
		require.NoError(t, err)
		require.Equal(t, block.Header.Height, latestFinalizedHeight)

		var headID flow.Identifier
		err = operation.LookupClusterBlockHeight(db.Reader(), block.Header.ChainID, latestFinalizedHeight, &headID)
		require.NoError(t, err)
		require.Equal(t, block.ID(), headID)
	})
}

// TestDisconnectedFinalizedBlock verifies that finalization logic rejects finalizing a block whose parent is not the latest finalized block.
func TestDisconnectedFinalizedBlock(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	lctx := lockManager.NewContext()
	require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
	defer lctx.Release()

	t.Run("finalizing C should fail because B is not yet finalized", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			_, _, blockC, _ := constructState(t, db, lctx)
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return FinalizeClusterBlock(lctx, rw, blockC.ID())
			})
			require.Error(t, err)
			require.NotErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

	t.Run("finalizing B and then C should succeed", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			_, blockB, blockC, _ := constructState(t, db, lctx)
			require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return FinalizeClusterBlock(lctx, rw, blockB.ID())
			}))
			require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return FinalizeClusterBlock(lctx, rw, blockC.ID())
			}))
		})
	})

	t.Run("finalizing B and then D should fail, because B is not the parent of D", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			_, blockB, _, blockD := constructState(t, db, lctx)
			require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return FinalizeClusterBlock(lctx, rw, blockB.ID())
			}))
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return FinalizeClusterBlock(lctx, rw, blockD.ID())
			})
			require.Error(t, err)
			require.NotErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

}

// `constructState` initializes a stub of the following collector chain state:
//
//	A ← B ← C
//	  ↖ D
func constructState(t *testing.T, db storage.DB, lctx lockctx.Proof) (blockA, blockB, blockC, blockD cluster.Block) {
	blockA = unittest.ClusterBlockFixture()
	blockB = unittest.ClusterBlockWithParent(&blockA)
	blockC = unittest.ClusterBlockWithParent(&blockB)
	blockD = unittest.ClusterBlockWithParent(&blockA)

	// Store all blocks
	for _, b := range []cluster.Block{blockA, blockB, blockC, blockD} {
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return InsertClusterBlock(lctx, rw, &b)
		}))
	}

	// index `blockA` as latest finalized block (manually writing respective indexes like in bootstrapping to skip transitive consistency checks)
	require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.IndexClusterBlockHeight(lctx, rw.Writer(), blockA.Header.ChainID, blockA.Header.Height, blockA.ID())
	}))
	require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.UpsertClusterFinalizedHeight(lctx, rw.Writer(), blockA.Header.ChainID, blockA.Header.Height)
	}))

	return blockA, blockB, blockC, blockD
}
