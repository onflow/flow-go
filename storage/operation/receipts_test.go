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

func TestReceipts_InsertRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		receipt := unittest.ExecutionReceiptFixture()
		expected := receipt.Stub()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertExecutionReceiptStub(rw.Writer(), receipt.ID(), expected)
		})
		require.Nil(t, err)

		var actual flow.ExecutionReceiptStub
		err = operation.RetrieveExecutionReceiptStub(db.Reader(), receipt.ID(), &actual)
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)
	})
}

func TestReceipts_Index(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		receipt := unittest.ExecutionReceiptFixture()
		expected := receipt.ID()
		blockID := receipt.ExecutionResult.BlockID

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexOwnExecutionReceipt(rw.Writer(), blockID, expected)
		})
		require.Nil(t, err)

		var actual flow.Identifier
		err = operation.LookupOwnExecutionReceipt(db.Reader(), blockID, &actual)
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestReceipts_MultiIndex(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		expected := []flow.Identifier{unittest.IdentifierFixture(), unittest.IdentifierFixture()}
		blockID := unittest.IdentifierFixture()

		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for _, id := range expected {
				err := operation.IndexExecutionReceipts(rw.Writer(), blockID, id)
				require.Nil(t, err)
			}
			return nil
		}))
		var actual []flow.Identifier
		err := operation.LookupExecutionReceipts(db.Reader(), blockID, &actual)
		require.Nil(t, err)

		assert.ElementsMatch(t, expected, actual)
	})
}
