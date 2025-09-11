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
func TestInstanceParams_InsertRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		enc, err := datastore.NewVersionedInstanceParams(
			datastore.DefaultInstanceParamsVersion,
			unittest.IdentifierFixture(),
			unittest.IdentifierFixture(),
			unittest.IdentifierFixture(),
		)
		require.NoError(t, err)

		lockManager := storage.NewTestingLockManager()
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
}
