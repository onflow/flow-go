package badger_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
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
				ErrorMessage:  fmt.Sprintf("a runtime error %d", i),
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
				ErrorMessage:  fmt.Sprintf("a runtime error %d", i),
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

func TestReadingNotStoreTransaction(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := bstorage.NewTransactionResults(db)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()

		_, err := store.ByBlockIDTransactionID(blockID, txID)
		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

func TestRemoveNotExistTransactionResult(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := bstorage.NewTransactionResults(db)
		blockID := unittest.IdentifierFixture()
		require.NoError(t, store.RemoveByBlockID(blockID))
	})
}

func TestRemoveTransactionResult(t *testing.T) {
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

		require.NoError(t, store.RemoveByBlockID(blockID), "remove for the first time should work, but didn't")

		result, err := store.ByBlockIDTransactionID(blockID, txResults[1].TransactionID)
		require.True(t, errors.Is(err, storage.ErrNotFound), fmt.Sprintf("%v", result))
	})
}
