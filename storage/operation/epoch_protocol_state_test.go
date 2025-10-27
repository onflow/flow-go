package operation_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInsertProtocolState tests if basic badger operations on EpochProtocolState work as expected.
func TestInsertEpochProtocolState(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		expected := unittest.EpochStateFixture().MinEpochStateEntry

		epochProtocolStateEntryID := expected.ID()
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertEpochProtocolState(rw.Writer(), epochProtocolStateEntryID, expected)
		})
		require.NoError(t, err)

		var actual flow.MinEpochStateEntry
		err = operation.RetrieveEpochProtocolState(db.Reader(), epochProtocolStateEntryID, &actual)
		require.NoError(t, err)

		assert.Equal(t, expected, &actual)

		blockID := unittest.IdentifierFixture()
		require.NoError(t, unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexEpochProtocolState(lctx, rw.Writer(), blockID, epochProtocolStateEntryID)
			})
		}))

		var actualProtocolStateID flow.Identifier
		err = operation.LookupEpochProtocolState(db.Reader(), blockID, &actualProtocolStateID)
		require.NoError(t, err)

		assert.Equal(t, epochProtocolStateEntryID, actualProtocolStateID)
	})
}

// TestIndexEpochProtocolStateLockValidation tests that IndexEpochProtocolState properly validates lock requirements.
func TestIndexEpochProtocolStateLockValidation(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		blockID := unittest.IdentifierFixture()
		epochProtocolStateEntryID := unittest.IdentifierFixture()

		t.Run("should error when no lock is held", func(t *testing.T) {
			// Create a context without any locks
			lctx := lockManager.NewContext()
			defer lctx.Release()

			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexEpochProtocolState(lctx, rw.Writer(), blockID, epochProtocolStateEntryID)
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing required lock")
			assert.Contains(t, err.Error(), storage.LockInsertBlock)
		})

		t.Run("should error when different lock is held", func(t *testing.T) {
			// Acquire a different lock (not LockInsertBlock)
			err := unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexEpochProtocolState(lctx, rw.Writer(), blockID, epochProtocolStateEntryID)
				})
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing required lock")
			assert.Contains(t, err.Error(), storage.LockInsertBlock)
		})

		t.Run("should succeed when correct lock is held", func(t *testing.T) {
			// This test case verifies that the function works correctly when the proper lock is held
			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexEpochProtocolState(lctx, rw.Writer(), blockID, epochProtocolStateEntryID)
				})
			})

			require.NoError(t, err)

			// Verify the indexing worked by looking up the entry
			var actualProtocolStateID flow.Identifier
			err = operation.LookupEpochProtocolState(db.Reader(), blockID, &actualProtocolStateID)
			require.NoError(t, err)
			assert.Equal(t, epochProtocolStateEntryID, actualProtocolStateID)
		})
	})
}
