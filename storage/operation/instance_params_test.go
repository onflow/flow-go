package operation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/datastore"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInstanceParams_InsertRetrieve verifies that InstanceParams can be
// correctly stored and retrieved from the database as a single encoded
// structure.
// Test cases:
//  1. InstanceParams can be inserted and retrieved successfully.
//  2. Overwrite attempts return storage.ErrAlreadyExists and do not change the
//     persisted value.
//  3. Writes without holding LockBootstrapping are denied.
func TestInstanceParams_InsertRetrieve(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	enc, err := datastore.NewVersionedInstanceParams(
		datastore.DefaultInstanceParamsVersion,
		unittest.IdentifierFixture(),
		unittest.IdentifierFixture(),
		unittest.IdentifierFixture(),
	)
	require.NoError(t, err)

	t.Run("happy path: insert and retrieve", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			lctx := lockManager.NewContext()
			require.NoError(t, lctx.AcquireLock(storage.LockBootstrapping))
			defer lctx.Release()

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertInstanceParams(lctx, rw, *enc)
			})
			require.NoError(t, err)

			var actual flow.VersionedInstanceParams
			err = operation.RetrieveInstanceParams(db.Reader(), &actual)
			require.NoError(t, err)
			require.Equal(t, enc, &actual)
		})
	})

	t.Run("overwrite returns ErrAlreadyExists", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			lctx := lockManager.NewContext()
			require.NoError(t, lctx.AcquireLock(storage.LockBootstrapping))

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertInstanceParams(lctx, rw, *enc)
			})
			require.NoError(t, err)
			lctx.Release()

			// try to overwrite with different params
			enc2, err := datastore.NewVersionedInstanceParams(
				datastore.DefaultInstanceParamsVersion,
				unittest.IdentifierFixture(),
				unittest.IdentifierFixture(),
				unittest.IdentifierFixture(),
			)
			require.NoError(t, err)

			lctx2 := lockManager.NewContext()
			require.NoError(t, lctx2.AcquireLock(storage.LockBootstrapping))
			defer lctx2.Release()

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertInstanceParams(lctx2, rw, *enc2)
			})
			require.ErrorIs(t, err, storage.ErrAlreadyExists)

			// DB must still contain original value
			var check flow.VersionedInstanceParams
			err = operation.RetrieveInstanceParams(db.Reader(), &check)
			require.NoError(t, err)
			require.Equal(t, enc, &check)
		})
	})

	t.Run("insert without required lock", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			lctx := lockManager.NewContext()
			defer lctx.Release()

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertInstanceParams(lctx, rw, *enc)
			})
			require.ErrorContains(t, err, storage.LockBootstrapping)
		})
	})
}
