package store_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestApprovalStoreAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		lockManager := storage.NewTestingLockManager()
		store := store.NewResultApprovals(metrics, db, lockManager)

		// create the deferred database operation to store `approval`; we deliberately
		// do this outside of the lock to confirm that the lock is not required for
		// creating the operation -- only for executing the storage write further below
		approval := unittest.ResultApprovalFixture()
		storing := store.StoreMyApproval(approval)
		err := unittest.WithLock(t, lockManager, storage.LockIndexResultApproval, func(lctx lockctx.Context) error {
			return storing(lctx)
		})
		require.NoError(t, err)

		// retrieve entire approval by its ID
		byID, err := store.ByID(approval.ID())
		require.NoError(t, err)
		require.Equal(t, approval, byID)

		// retrieve approval by pair (executed result ID, chunk index)
		byChunk, err := store.ByChunk(approval.Body.ExecutionResultID, approval.Body.ChunkIndex)
		require.NoError(t, err)
		require.Equal(t, approval, byChunk)
	})
}

func TestApprovalStoreTwice(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		lockManager := storage.NewTestingLockManager()
		store := store.NewResultApprovals(metrics, db, lockManager)

		// create the deferred database operation to store `approval`; we deliberately
		// do this outside of the lock to confirm that the lock is not required for
		// creating the operation -- only for executing the storage write further below
		approval := unittest.ResultApprovalFixture()
		storing := store.StoreMyApproval(approval)
		err := unittest.WithLock(t, lockManager, storage.LockIndexResultApproval, func(lctx lockctx.Context) error {
			return storing(lctx)
		})
		require.NoError(t, err)

		err = unittest.WithLock(t, lockManager, storage.LockIndexResultApproval, func(lctx lockctx.Context) error {
			return storing(lctx) // repeated storage of same approval should be no-op
		})
		require.NoError(t, err)
	})
}

func TestApprovalStoreTwoDifferentApprovalsShouldFail(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		lockManager := storage.NewTestingLockManager()
		store := store.NewResultApprovals(metrics, db, lockManager)

		approval1, approval2 := twoApprovalsForTheSameResult(t)

		storing := store.StoreMyApproval(approval1)
		err := unittest.WithLock(t, lockManager, storage.LockIndexResultApproval, func(lctx lockctx.Context) error {
			return storing(lctx)
		})
		require.NoError(t, err)

		// we can store a different approval, but we can't index a different
		// approval for the same chunk.
		storing2 := store.StoreMyApproval(approval2)
		err = unittest.WithLock(t, lockManager, storage.LockIndexResultApproval, func(lctx lockctx.Context) error {
			return storing2(lctx)
		})
		require.ErrorIs(t, err, storage.ErrDataMismatch)
	})
}

// verify that storing and indexing two conflicting approvals concurrently should be impossible;
// we expect that one operations succeeds, the other one should fail
func TestApprovalStoreTwoDifferentApprovalsConcurrently(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		lockManager := storage.NewTestingLockManager()
		store := store.NewResultApprovals(metrics, db, lockManager)

		approval1, approval2 := twoApprovalsForTheSameResult(t)

		var startSignal sync.WaitGroup // goroutines attempting store operations will wait for this signal to start concurrently
		startSignal.Add(1)             // expecting one signal from the main thread to start both goroutines
		var doneSinal sync.WaitGroup   // the main thread will wait on this for goroutines attempting store operations to finish
		doneSinal.Add(2)               // expecting two goroutines to signal finish

		var firstIndexErr, secondIndexErr error

		// First goroutine stores and indexes the first approval.
		go func() {
			storing := store.StoreMyApproval(approval1)

			startSignal.Wait()
			err := unittest.WithLock(t, lockManager, storage.LockIndexResultApproval, func(lctx lockctx.Context) error {
				firstIndexErr = storing(lctx)
				return nil
			})
			require.NoError(t, err)
			doneSinal.Done()
		}()

		// Second goroutine stores and tries to index the second approval for the same chunk.
		go func() {
			storing := store.StoreMyApproval(approval2)

			startSignal.Wait()
			err := unittest.WithLock(t, lockManager, storage.LockIndexResultApproval, func(lctx lockctx.Context) error {
				secondIndexErr = storing(lctx)
				return nil
			})
			require.NoError(t, err)
			doneSinal.Done()
		}()

		startSignal.Done() // start both goroutines
		doneSinal.Wait()   // wait for both goroutines to finish

		// Check that one of the Index operations succeeded and the other failed
		if firstIndexErr == nil {
			require.Error(t, secondIndexErr)
			require.True(t, errors.Is(secondIndexErr, storage.ErrDataMismatch))
		} else {
			require.NoError(t, secondIndexErr)
			require.True(t, errors.Is(firstIndexErr, storage.ErrDataMismatch))
		}
	})
}

func twoApprovalsForTheSameResult(t *testing.T) (*flow.ResultApproval, *flow.ResultApproval) {
	approval1 := unittest.ResultApprovalFixture()
	approval2 := unittest.ResultApprovalFixture()
	// have two entirely different approvals, nor modify the second to reference the same result and chunk as the first
	approval2.Body.ChunkIndex = approval1.Body.ChunkIndex
	approval2.Body.ExecutionResultID = approval1.Body.ExecutionResultID
	// sanity check: make sure the two approvals are different
	require.NotEqual(t, approval1.ID(), approval2.ID(), "expected two different approvals, but got the same ID")
	return approval1, approval2
}
