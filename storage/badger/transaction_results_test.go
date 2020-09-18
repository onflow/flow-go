package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	bstorage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestStoringTransactionResults(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := bstorage.NewTransactionResults(db)

		blockID := unittest.IdentifierFixture()
		txResults := make([]*flow.TransactionResult, 0)
		for i := 0; i < 10; i++ {
			txID := unittest.IdentifierFixture()
			expected := &flow.TransactionResult{
				TransactionID: txID,
				ErrorMessage:  "a runtime error " + string(i),
			}
			txResults = append(txResults, expected)
		}

		for _, txResult := range txResults {
			err := store.Store(blockID, txResult)
			require.Nil(t, err)
		}
		for _, txResult := range txResults {
			actual, err := store.ByBlockIDTransactionID(blockID, txResult.TransactionID)
			require.Nil(t, err)
			assert.Equal(t, txResult, actual)
		}
	})
}

func TestBatchStoringTransactionResults(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := bstorage.NewTransactionResults(db)

		blockID := unittest.IdentifierFixture()
		txResults := make([]flow.TransactionResult, 0)
		for i := 0; i < 10; i++ {
			txID := unittest.IdentifierFixture()
			expected := flow.TransactionResult{
				TransactionID: txID,
				ErrorMessage:  "a runtime error " + string(i),
			}
			txResults = append(txResults, expected)
		}
		err := store.BatchStore(blockID, txResults)
		require.Nil(t, err)

		for _, txResult := range txResults {
			actual, err := store.ByBlockIDTransactionID(blockID, txResult.TransactionID)
			require.Nil(t, err)
			assert.Equal(t, txResult, *actual)
		}
	})
}
