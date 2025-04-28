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
		expected := unittest.StateCommitmentFixture()
		id := unittest.IdentifierFixture()
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexStateCommitment(rw.Writer(), id, expected)
		}))

		reader, err := db.Reader()
		require.NoError(t, err)

		var actual flow.StateCommitment
		err = operation.LookupStateCommitment(reader, id, &actual)
		require.Nil(t, err)
		require.Equal(t, expected, actual)
	})
}
