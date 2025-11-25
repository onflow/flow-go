package operation_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"
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

		err := unittest.WithLock(t, lockManager, storage.LockIndexStateCommitment, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexStateCommitment(lctx, rw, id, expected)
			})
		})
		require.NoError(t, err)

		var actual flow.StateCommitment
		err = operation.LookupStateCommitment(db.Reader(), id, &actual)
		require.Nil(t, err)
		require.Equal(t, expected, actual)
	})
}
