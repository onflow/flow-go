package store_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestApprovalStoreAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := store.NewResultApprovals(metrics, db)

		lockManager := storage.NewTestingLockManager()
		lctx := lockManager.NewContext()
		defer lctx.Release()
		require.NoError(t, lctx.AcquireLock(storage.LockIndexResultApproval))
		approval := unittest.ResultApprovalFixture()
		err := store.Store(lctx, approval)
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

		lockManager := storage.NewTestingLockManager()
		lctx := lockManager.NewContext()
		defer lctx.Release()
		require.NoError(t, lctx.AcquireLock(storage.LockIndexResultApproval))
		approval := unittest.ResultApprovalFixture()
		err := store.Store(lctx, approval)
		require.NoError(t, err)

		err = store.Store(lctx, approval)
		require.NoError(t, err)
	})
}

func TestApprovalStoreTwoDifferentApprovalsShouldFail(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := store.NewResultApprovals(metrics, db)

		approval1 := unittest.ResultApprovalFixture()
		approval2 := unittest.ResultApprovalFixture()

		lockManager := storage.NewTestingLockManager()
		lctx := lockManager.NewContext()
		defer lctx.Release()
		require.NoError(t, lctx.AcquireLock(storage.LockIndexResultApproval))
		err := store.Store(lctx, approval1)
		require.NoError(t, err)

		// we can store a different approval, but we can't index a different
		// approval for the same chunk.
		err = store.Store(lctx, approval2)
		require.ErrorIs(t, err, storage.ErrDataMismatch)
	})
}

// verify that storing and indexing two conflicting approvals concurrently should be impossible;
// we expect that one operations succeeds, the other one should fail
func TestApprovalStoreTwoDifferentApprovalsConcurrently(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := store.NewResultApprovals(metrics, db)

		lockManager := storage.NewTestingLockManager()
		approval1 := unittest.ResultApprovalFixture()
		approval2 := unittest.ResultApprovalFixture()

		var wg sync.WaitGroup
		wg.Add(2)

		var firstIndexErr, secondIndexErr error

		// First goroutine stores and indexes the first approval.
		go func() {
			defer wg.Done()

			lctx := lockManager.NewContext()
			defer lctx.Release()
			require.NoError(t, lctx.AcquireLock(storage.LockIndexResultApproval))

			firstIndexErr = store.Store(lctx, approval1)
		}()

		// Second goroutine stores and tries to index the second approval for the same chunk.
		go func() {
			defer wg.Done()

			lctx := lockManager.NewContext()
			defer lctx.Release()
			require.NoError(t, lctx.AcquireLock(storage.LockIndexResultApproval))

			secondIndexErr = store.Store(lctx, approval2)
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
