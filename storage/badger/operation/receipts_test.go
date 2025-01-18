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
		expected := receipt.Meta()

		err := db.Update(InsertExecutionReceiptMeta(receipt.ID(), expected))
		require.Nil(t, err)

		var actual flow.ExecutionReceiptMeta
		err = db.View(RetrieveExecutionReceiptMeta(receipt.ID(), &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)
	})
}

func TestReceipts_Index(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		receipt := unittest.ExecutionReceiptFixture()
		expected := receipt.ID()
		blockID := receipt.ExecutionResult.BlockID

		err := db.Update(IndexOwnExecutionReceipt(blockID, expected))
		require.Nil(t, err)

		var actual flow.Identifier
		err = db.View(LookupOwnExecutionReceipt(blockID, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestReceipts_MultiIndex(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := []flow.Identifier{unittest.IdentifierFixture(), unittest.IdentifierFixture()}
		blockID := unittest.IdentifierFixture()

		for _, id := range expected {
			err := db.Update(IndexExecutionReceipts(blockID, id))
			require.Nil(t, err)
		}
		var actual []flow.Identifier
		err := db.View(LookupExecutionReceipts(blockID, &actual))
		require.Nil(t, err)

		assert.ElementsMatch(t, expected, actual)
	})
}
