package store_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestExecutionForkEvidenceStoreAndRetrieve(t *testing.T) {
	t.Run("Retrieving non-existing evidence should return no error", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			evidenceStore := store.NewExecutionForkEvidence(db)

			conflictingSeals, err := evidenceStore.Retrieve()
			require.NoError(t, err)
			require.Equal(t, 0, len(conflictingSeals))
		})
	})

	t.Run("Store and read evidence", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			lockManager := storage.NewTestingLockManager()
			block := unittest.BlockFixture()

			conflictingSeals := make([]*flow.IncorporatedResultSeal, 2)
			for i := range len(conflictingSeals) {
				conflictingSeals[i] = unittest.IncorporatedResultSeal.Fixture(
					unittest.IncorporatedResultSeal.WithResult(
						unittest.ExecutionResultFixture(
							unittest.WithBlock(block))))
			}

			evidenceStore := store.NewExecutionForkEvidence(db)

			err := unittest.WithLock(t, lockManager, storage.LockInsertExecutionForkEvidence, func(lctx lockctx.Context) error {
				return evidenceStore.StoreIfNotExists(lctx, conflictingSeals)
			})
			require.NoError(t, err)

			retrievedConflictingSeals, err := evidenceStore.Retrieve()
			require.NoError(t, err)
			require.Equal(t, conflictingSeals, retrievedConflictingSeals)
		})
	})

	t.Run("Don't overwrite evidence", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			lockManager := storage.NewTestingLockManager()
			block := unittest.BlockFixture()

			conflictingSeals := make([]*flow.IncorporatedResultSeal, 2)
			for i := range len(conflictingSeals) {
				conflictingSeals[i] = unittest.IncorporatedResultSeal.Fixture(
					unittest.IncorporatedResultSeal.WithResult(
						unittest.ExecutionResultFixture(
							unittest.WithBlock(block))))
			}

			conflictingSeals2 := make([]*flow.IncorporatedResultSeal, 2)
			for i := range len(conflictingSeals2) {
				conflictingSeals2[i] = unittest.IncorporatedResultSeal.Fixture(
					unittest.IncorporatedResultSeal.WithResult(
						unittest.ExecutionResultFixture(
							unittest.WithBlock(block))))
			}

			evidenceStore := store.NewExecutionForkEvidence(db)

			// Store and read evidence.
			{
				err := unittest.WithLock(t, lockManager, storage.LockInsertExecutionForkEvidence, func(lctx lockctx.Context) error {
					return evidenceStore.StoreIfNotExists(lctx, conflictingSeals)
				})
				require.NoError(t, err)

				retrievedConflictingSeals, err := evidenceStore.Retrieve()
				require.NoError(t, err)
				require.Equal(t, conflictingSeals, retrievedConflictingSeals)
			}

			// Overwriting existing evidence is no-op.
			{
				err := unittest.WithLock(t, lockManager, storage.LockInsertExecutionForkEvidence, func(lctx lockctx.Context) error {
					return evidenceStore.StoreIfNotExists(lctx, conflictingSeals2)
				})
				require.NoError(t, err)

				retrievedConflictingSeals, err := evidenceStore.Retrieve()
				require.NoError(t, err)
				require.Equal(t, conflictingSeals, retrievedConflictingSeals)
				require.NotEqual(t, conflictingSeals2, retrievedConflictingSeals)
			}
		})
	})
}
