package operation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInsertQuorumCertificate(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		expected := unittest.QuorumCertificateFixture()
		lockManager := storage.NewTestingLockManager()

		lctx := lockManager.NewContext()
		require.NoError(t, lctx.AcquireLock(storage.LockInsertBlock))
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertQuorumCertificate(lctx, rw, expected)
		})
		require.NoError(t, err)

		// While still holding the lock, get value; this verifies that reads are not blocked by acquired locks
		var actual flow.QuorumCertificate
		err = operation.RetrieveQuorumCertificate(db.Reader(), expected.BlockID, &actual)
		require.NoError(t, err)
		assert.Equal(t, expected, &actual)
		lctx.Release()

		// create a different QC for the same block
		different := unittest.QuorumCertificateFixture()
		different.BlockID = expected.BlockID

		// verify that overwriting the prior QC fails with `storage.ErrAlreadyExists`
		lctx2 := lockManager.NewContext()
		require.NoError(t, lctx2.AcquireLock(storage.LockInsertBlock))
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertQuorumCertificate(lctx2, rw, different)
		})
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
		lctx2.Release()

		// verify that the original QC is still there
		err = operation.RetrieveQuorumCertificate(db.Reader(), expected.BlockID, &actual)
		require.NoError(t, err)
		assert.Equal(t, expected, &actual)
	})
}
