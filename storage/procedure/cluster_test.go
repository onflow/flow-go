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

		lockManager := storage.NewTestingLockManager()
		err := unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return InsertClusterBlock(lctx, rw, unittest.ClusterProposalFromBlock(block))
			})
		})
		require.NoError(t, err)

		var retrieved cluster.Block
		err = RetrieveClusterBlock(db.Reader(), block.ID(), &retrieved)
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
		err := unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
			require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return InsertClusterBlock(lctx, rw, unittest.ClusterProposalFromBlock(parent))
			}))

			// index parent as latest finalized block (manually writing respective indexes like in bootstrapping to skip transitive consistency checks)
			require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexClusterBlockHeight(lctx, rw.Writer(), block.ChainID, parent.Height, parent.ID())
			}))
			require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertClusterFinalizedHeight(lctx, rw.Writer(), block.ChainID, parent.Height)
			}))

			// Insert new block and verify `FinalizeClusterBlock` procedure accepts it
			require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return InsertClusterBlock(lctx, rw, unittest.ClusterProposalFromBlock(block))
			}))
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return FinalizeClusterBlock(lctx, rw, block.ID())
			})
		})
		require.NoError(t, err)

		// verify that the new block as been properly indexed as the latest finalized
		var latestFinalizedHeight uint64
		err = operation.RetrieveClusterFinalizedHeight(db.Reader(), block.ChainID, &latestFinalizedHeight)
		require.NoError(t, err)
		require.Equal(t, block.Height, latestFinalizedHeight)

		var headID flow.Identifier
		err = operation.LookupClusterBlockHeight(db.Reader(), block.ChainID, latestFinalizedHeight, &headID)
		require.NoError(t, err)
		require.Equal(t, block.ID(), headID)
	})
}

// TestDisconnectedFinalizedBlock verifies that finalization logic rejects finalizing a block whose parent is not the latest finalized block.
func TestDisconnectedFinalizedBlock(t *testing.T) {
	lockManager := storage.NewTestingLockManager()

	t.Run("finalizing C should fail because B is not yet finalized", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			err := unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
				_, _, blockC, _ := constructState(t, db, lctx)
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return FinalizeClusterBlock(lctx, rw, blockC.ID())
				})
			})
			require.Error(t, err)
			require.NotErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

	t.Run("finalizing B and then C should succeed", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			err := unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
				_, blockB, blockC, _ := constructState(t, db, lctx)
				require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return FinalizeClusterBlock(lctx, rw, blockB.ID())
				}))
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return FinalizeClusterBlock(lctx, rw, blockC.ID())
				})
			})
			require.NoError(t, err)
		})
	})

	t.Run("finalizing B and then D should fail, because B is not the parent of D", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			err := unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
				_, blockB, _, blockD := constructState(t, db, lctx)
				require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return FinalizeClusterBlock(lctx, rw, blockB.ID())
				}))
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return FinalizeClusterBlock(lctx, rw, blockD.ID())
				})

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
func constructState(t *testing.T, db storage.DB, lctx lockctx.Proof) (blockA, blockB, blockC, blockD *cluster.Block) {
	blockA = unittest.ClusterBlockFixture()                                         // Create block A as the root
	blockB = unittest.ClusterBlockFixture(unittest.ClusterBlock.WithParent(blockA)) // Create block B as a child of A
	blockC = unittest.ClusterBlockFixture(unittest.ClusterBlock.WithParent(blockB)) // Create block C as a child of B
	blockD = unittest.ClusterBlockFixture(unittest.ClusterBlock.WithParent(blockA)) // Create block D as a child of A (creating a fork)

	// Store all blocks
	for _, b := range []*cluster.Block{blockA, blockB, blockC, blockD} {
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return InsertClusterBlock(lctx, rw, unittest.ClusterProposalFromBlock(b))
		}))
	}

	// index `blockA` as latest finalized block (manually writing respective indexes like in bootstrapping to skip transitive consistency checks)
	require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.IndexClusterBlockHeight(lctx, rw.Writer(), blockA.ChainID, blockA.Height, blockA.ID())
	}))
	require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.UpsertClusterFinalizedHeight(lctx, rw.Writer(), blockA.ChainID, blockA.Height)
	}))

	return blockA, blockB, blockC, blockD
}
