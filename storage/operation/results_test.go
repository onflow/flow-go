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

func TestResults_InsertRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		expected := unittest.ExecutionResultFixture()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertExecutionResult(rw.Writer(), expected)
		})
		require.Nil(t, err)

		reader, err := db.Reader()
		require.NoError(t, err)

		var actual flow.ExecutionResult
		err = operation.RetrieveExecutionResult(reader, expected.ID(), &actual)
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)
	})
}
