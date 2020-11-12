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

func TestReceiptStoreAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		results := bstorage.NewExecutionResults(db)
		store := bstorage.NewExecutionReceipts(db, results)

		receipt := unittest.ExecutionReceiptFixture()
		blockID := unittest.IdentifierFixture()
		err := store.Store(receipt)
		require.NoError(t, err)

		err = store.Index(blockID, receipt.ID())
		require.NoError(t, err)

		actual, err := store.ByBlockID(blockID)
		require.NoError(t, err)

		require.Equal(t, receipt, actual)
	})
}

func TestReceiptStoreTwice(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		results := bstorage.NewExecutionResults(db)
		store := bstorage.NewExecutionReceipts(db, results)

		receipt := unittest.ExecutionReceiptFixture()
		blockID := unittest.IdentifierFixture()
		err := store.Store(receipt)
		require.NoError(t, err)

		err = store.Index(blockID, receipt.ID())
		require.NoError(t, err)

		err = store.Store(receipt)
		require.NoError(t, err)

		err = store.Index(blockID, receipt.ID())
		require.NoError(t, err)
	})
}

func TestReceiptStoreTwoDifferentReceiptsShouldFail(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		results := bstorage.NewExecutionResults(db)
		store := bstorage.NewExecutionReceipts(db, results)

		receipt1 := unittest.ExecutionReceiptFixture()
		receipt2 := unittest.ExecutionReceiptFixture()
		blockID := unittest.IdentifierFixture()
		err := store.Store(receipt1)
		require.NoError(t, err)

		err = store.Index(blockID, receipt1.ID())
		require.NoError(t, err)

		// we can store a different receipt, but we can't index
		// a different receipt for that block, because it will mean
		// one block has two different receipts.
		err = store.Store(receipt2)
		require.NoError(t, err)

		err = store.Index(blockID, receipt2.ID())
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrDataMismatch))
	})
}

func TestReceiptStoreTwoDifferentReceiptsShouldOKIfResultAreSame(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		results := bstorage.NewExecutionResults(db)
		store := bstorage.NewExecutionReceipts(db, results)

		receipt1 := unittest.ExecutionReceiptFixture()
		receipt2 := unittest.ExecutionReceiptFixture()
		receipt2.ExecutionResult = receipt1.ExecutionResult

		blockID := unittest.IdentifierFixture()
		err := store.Store(receipt1)
		require.NoError(t, err)

		err = store.Index(blockID, receipt1.ID())
		require.NoError(t, err)

		err = store.Store(receipt2)
		require.NoError(t, err)

		err = store.Index(blockID, receipt2.ID())
		require.NoError(t, err)
	})
}
