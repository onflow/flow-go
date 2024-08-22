package badger_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestApprovalStoreAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewResultApprovals(metrics, db)

		approval := unittest.ResultApprovalFixture()
		err := store.Store(approval)
		require.NoError(t, err)

		err = store.Index(approval.Body.ExecutionResultID, approval.Body.ChunkIndex, approval.ID())
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
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewResultApprovals(metrics, db)

		approval := unittest.ResultApprovalFixture()
		err := store.Store(approval)
		require.NoError(t, err)

		err = store.Index(approval.Body.ExecutionResultID, approval.Body.ChunkIndex, approval.ID())
		require.NoError(t, err)

		err = store.Store(approval)
		require.NoError(t, err)

		err = store.Index(approval.Body.ExecutionResultID, approval.Body.ChunkIndex, approval.ID())
		require.NoError(t, err)
	})
}

func TestApprovalStoreTwoDifferentApprovalsShouldFail(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewResultApprovals(metrics, db)

		approval1 := unittest.ResultApprovalFixture()
		approval2 := unittest.ResultApprovalFixture()

		err := store.Store(approval1)
		require.NoError(t, err)

		err = store.Index(approval1.Body.ExecutionResultID, approval1.Body.ChunkIndex, approval1.ID())
		require.NoError(t, err)

		// we can store a different approval, but we can't index a different
		// approval for the same chunk.
		err = store.Store(approval2)
		require.NoError(t, err)

		err = store.Index(approval1.Body.ExecutionResultID, approval1.Body.ChunkIndex, approval2.ID())
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrDataMismatch))
	})
}

// verify that storing and indexing two conflicting approvals concurrently should fail
// one of them is succeed, the other one should fail
func TestApprovalStoreTwoDifferentApprovalsConcurrently(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewResultApprovals(metrics, db)

		approval1 := unittest.ResultApprovalFixture()
		approval2 := unittest.ResultApprovalFixture()

		var wg sync.WaitGroup
		wg.Add(2)

		var firstIndexErr, secondIndexErr error

		// First goroutine stores and indexes the first approval.
		go func() {
			defer wg.Done()

			err := store.Store(approval1)
			require.NoError(t, err)

			firstIndexErr = store.Index(approval1.Body.ExecutionResultID, approval1.Body.ChunkIndex, approval1.ID())
		}()

		// Second goroutine stores and tries to index the second approval for the same chunk.
		go func() {
			defer wg.Done()

			err := store.Store(approval2)
			require.NoError(t, err)

			secondIndexErr = store.Index(approval1.Body.ExecutionResultID, approval1.Body.ChunkIndex, approval2.ID())
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
