package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReceiptsRemoveNotExist(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		results := bstorage.NewExecutionResults(db)
		store := bstorage.NewExecutionReceipts(db, results)
		blockID := unittest.IdentifierFixture()
		require.NoError(t, store.RemoveByBlockID(blockID))
	})
}

func TestReceiptsRemove(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		results := bstorage.NewExecutionResults(db)
		store := bstorage.NewExecutionReceipts(db, results)

		receipt := unittest.ExecutionReceiptFixture()
		err := store.Store(receipt)
		require.Nil(t, err)

		// this is redundant, Store should be able to Index a receipt
		require.NoError(t, store.Index(receipt.ExecutionResult.BlockID, receipt.ID()))

		require.NoError(t, store.RemoveByBlockID(receipt.ExecutionResult.BlockID))

		_, err = store.ByBlockID(receipt.ExecutionResult.BlockID)
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}
