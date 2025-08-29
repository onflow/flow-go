package operation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestStateCommitments(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		expected := unittest.StateCommitmentFixture()
		id := unittest.IdentifierFixture()
		lctx := lockManager.NewContext()
		defer lctx.Release()
		require.NoError(t, lctx.AcquireLock(storage.LockInsertOwnReceipt))
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexStateCommitment(lctx, rw, id, expected)
		}))

		var actual flow.StateCommitment
		err := operation.LookupStateCommitment(db.Reader(), id, &actual)
		require.Nil(t, err)
		require.Equal(t, expected, actual)
	})
}
