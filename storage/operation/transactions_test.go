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
		// storing a tx
		expected := unittest.TransactionFixture()
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertTransaction(rw.Writer(), expected.ID(), &expected.TransactionBody)
		})
		require.NoError(t, err)

		// verify can be retrieved
		var actual flow.Transaction
		err = operation.RetrieveTransaction(db.Reader(), expected.ID(), &actual.TransactionBody)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// retrieve non exist
		err = operation.RetrieveTransaction(db.Reader(), unittest.IdentifierFixture(), &actual.TransactionBody)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// delete
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.RemoveTransaction(rw.Writer(), expected.ID())
		})
		require.NoError(t, err)

		// verify has been deleted
		err = operation.RetrieveTransaction(db.Reader(), expected.ID(), &actual.TransactionBody)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// deleting a non exist
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.RemoveTransaction(rw.Writer(), unittest.IdentifierFixture())
		})
		require.NoError(t, err)
	})
}
