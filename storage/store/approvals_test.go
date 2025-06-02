package store_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/locks"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestApprovalStoreAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := store.NewResultApprovals(metrics, db)

		lockManager := locks.NewTestingLockManager()
		lctx := lockManager.NewContext()
		defer lctx.Release()
		require.NoError(t, lctx.AcquireLock(storage.LockMyResultApproval))
		approval := unittest.ResultApprovalFixture()
		err := store.StoreMyApproval(lctx, approval)
		require.NoError(t, err)

		byID, err := store.ByID(approval.ID())
		require.NoError(t, err)
		require.Equal(t, approval, byID)

		byChunk, err := store.ByChunk(approval.Body.ExecutionResultID, approval.Body.ChunkIndex)
		require.NoError(t, err)
		require.Equal(t, approval, byChunk)
	})
}

func TestApprovalStoreTwice(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := store.NewResultApprovals(metrics, db)

		lockManager := locks.NewTestingLockManager()
		lctx := lockManager.NewContext()
		defer lctx.Release()
		require.NoError(t, lctx.AcquireLock(storage.LockMyResultApproval))
		approval := unittest.ResultApprovalFixture()
		err := store.StoreMyApproval(lctx, approval)
		require.NoError(t, err)

		err = store.StoreMyApproval(lctx, approval)
		require.NoError(t, err)
	})
}

func TestApprovalStoreTwoDifferentApprovalsShouldFail(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := store.NewResultApprovals(metrics, db)

		approval1, approval2 := twoApprovalsForTheSameResult()

		lockManager := locks.NewTestingLockManager()
		lctx := lockManager.NewContext()
		require.NoError(t, lctx.AcquireLock(storage.LockMyResultApproval))

		err := store.StoreMyApproval(lctx, approval1)
		lctx.Release()
		require.NoError(t, err)

		// we can store a different approval, but we can't index a different
		// approval for the same chunk.
		lctx2 := lockManager.NewContext()
		require.NoError(t, lctx2.AcquireLock(storage.LockMyResultApproval))
		err = store.StoreMyApproval(lctx2, approval2)
		lctx2.Release()
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrDataMismatch)
	})
}

// verify that storing and indexing two conflicting approvals concurrently should be impossible;
// we expect that one operations succeeds, the other one should fail
func TestApprovalStoreTwoDifferentApprovalsConcurrently(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := store.NewResultApprovals(metrics, db)

		lockManager := locks.NewTestingLockManager()
		approval1, approval2 := twoApprovalsForTheSameResult()

		var wg sync.WaitGroup
		wg.Add(2)

		var firstIndexErr, secondIndexErr error

		// First goroutine stores and indexes the first approval.
		go func() {
			defer wg.Done()

			lctx := lockManager.NewContext()
			defer lctx.Release()
			require.NoError(t, lctx.AcquireLock(storage.LockMyResultApproval))

			firstIndexErr = store.StoreMyApproval(lctx, approval1)
		}()

		// Second goroutine stores and tries to index the second approval for the same chunk.
		go func() {
			defer wg.Done()

			lctx := lockManager.NewContext()
			defer lctx.Release()
			require.NoError(t, lctx.AcquireLock(storage.LockMyResultApproval))

			secondIndexErr = store.StoreMyApproval(lctx, approval2)
		}()

		// Wait for both goroutines to finish
		wg.Wait()

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

func twoApprovalsForTheSameResult() (*flow.ResultApproval, *flow.ResultApproval) {
	approval1 := unittest.ResultApprovalFixture()
	approval2 := unittest.ResultApprovalFixture()
	// make sure the two approvals are different
	approval2.Body.ChunkIndex = approval1.Body.ChunkIndex
	approval2.Body.ExecutionResultID = approval1.Body.ExecutionResultID
	return approval1, approval2
}
