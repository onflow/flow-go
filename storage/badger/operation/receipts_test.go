package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReceipts_InsertRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		receipt := unittest.ExecutionReceiptFixture()
		expected := receipt.Stub()

		err := db.Update(InsertExecutionReceiptStub(receipt.ID(), expected))
		require.NoError(t, err)

		var actual flow.ExecutionReceiptStub
		err = db.View(RetrieveExecutionReceiptStub(receipt.ID(), &actual))
		require.NoError(t, err)

		assert.Equal(t, expected, &actual)
	})
}

func TestReceipts_Index(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		receipt := unittest.ExecutionReceiptFixture()
		expected := receipt.ID()
		blockID := receipt.ExecutionResult.BlockID

		err := db.Update(IndexOwnExecutionReceipt(blockID, expected))
		require.NoError(t, err)

		var actual flow.Identifier
		err = db.View(LookupOwnExecutionReceipt(blockID, &actual))
		require.NoError(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestReceipts_MultiIndex(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := []flow.Identifier{unittest.IdentifierFixture(), unittest.IdentifierFixture()}
		blockID := unittest.IdentifierFixture()

		for _, id := range expected {
			err := db.Update(IndexExecutionReceipts(blockID, id))
			require.NoError(t, err)
		}
		var actual []flow.Identifier
		err := db.View(LookupExecutionReceipts(blockID, &actual))
		require.NoError(t, err)

		assert.ElementsMatch(t, expected, actual)
	})
}
