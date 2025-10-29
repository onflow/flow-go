package operation_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/stretchr/testify/require"
)

func Test_ExecutionForkEvidenceOperations(t *testing.T) {
	t.Run("Retrieving non-existing evidence should return 'storage.ErrNotFound'", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			var conflictingSeals []*flow.IncorporatedResultSeal
			err := operation.RetrieveExecutionForkEvidence(db.Reader(), &conflictingSeals)
			require.ErrorIs(t, err, storage.ErrNotFound)

			exists, err := operation.HasExecutionForkEvidence(db.Reader())
			require.NoError(t, err)
			require.False(t, exists)
		})
	})

	t.Run("Write evidence and retrieve", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			lockManager := storage.NewTestingLockManager()
			exists, err := operation.HasExecutionForkEvidence(db.Reader())
			require.NoError(t, err)
			require.False(t, exists)

			block := unittest.BlockFixture()

			conflictingSeals := make([]*flow.IncorporatedResultSeal, 2)
			for i := range len(conflictingSeals) {
				conflictingSeals[i] = unittest.IncorporatedResultSeal.Fixture(
					unittest.IncorporatedResultSeal.WithResult(
						unittest.ExecutionResultFixture(
							unittest.WithBlock(block))))
			}

			err = unittest.WithLock(t, lockManager, storage.LockInsertExecutionForkEvidence, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.InsertExecutionForkEvidence(lctx, rw, conflictingSeals)
				})
			})
			require.NoError(t, err)

			var b []*flow.IncorporatedResultSeal
			err = operation.RetrieveExecutionForkEvidence(db.Reader(), &b)
			require.NoError(t, err)
			require.Equal(t, conflictingSeals, b)

			exists, err = operation.HasExecutionForkEvidence(db.Reader())
			require.NoError(t, err)
			require.True(t, exists)
		})
	})
}
