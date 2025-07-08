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

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UnsafeUpsertQuorumCertificate(rw.Writer(), expected)
		})
		require.NoError(t, err)

		var actual flow.QuorumCertificate
		err = operation.RetrieveQuorumCertificate(db.Reader(), expected.BlockID, &actual)
		require.NoError(t, err)

		assert.Equal(t, expected, &actual)

		// create a different QC for the same block
		different := unittest.QuorumCertificateFixture()
		different.BlockID = expected.BlockID

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UnsafeUpsertQuorumCertificate(rw.Writer(), different)
		})
		require.NoError(t, err)

		err = operation.RetrieveQuorumCertificate(db.Reader(), expected.BlockID, &actual)
		require.NoError(t, err)

		assert.Equal(t, different, &actual)
	})
}
