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

func TestTransactions(t *testing.T) {

	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		expected := unittest.TransactionFixture()
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertTransaction(rw.Writer(), expected.ID(), &expected.TransactionBody)
		})
		require.Nil(t, err)

		var actual flow.Transaction
		err = operation.RetrieveTransaction(db.Reader(), expected.ID(), &actual.TransactionBody)
		require.Nil(t, err)
		assert.Equal(t, expected, actual)
	})
}
