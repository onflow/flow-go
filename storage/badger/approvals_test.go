package badger_test

import (
	"errors"
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
