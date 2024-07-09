package operation

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReceipts_InsertRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		receipt := unittest.ExecutionReceiptFixture()
		expected := receipt.Meta()

		err := InsertExecutionReceiptMeta(receipt.ID(), expected)(db)
		require.Nil(t, err)

		var actual flow.ExecutionReceiptMeta
		err = RetrieveExecutionReceiptMeta(receipt.ID(), &actual)(db)
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)
	})
}

func TestReceipts_Index(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		receipt := unittest.ExecutionReceiptFixture()
		expected := receipt.ID()
		blockID := receipt.ExecutionResult.BlockID

		err := IndexOwnExecutionReceipt(blockID, expected)(db)
		require.Nil(t, err)

		var actual flow.Identifier
		err = LookupOwnExecutionReceipt(blockID, &actual)(db)
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestReceipts_MultiIndex(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		expected := []flow.Identifier{unittest.IdentifierFixture(), unittest.IdentifierFixture()}
		blockID := unittest.IdentifierFixture()

		for _, id := range expected {
			err := IndexExecutionReceipts(blockID, id)(db)
			require.Nil(t, err)
		}
		var actual []flow.Identifier
		err := LookupExecutionReceipts(blockID, &actual)(db)
		require.Nil(t, err)

		assert.ElementsMatch(t, expected, actual)
	})
}
