package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
)

func TestTransactionStoreRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewTransactions(metrics, db)

		// store a transaction in db
		expected := unittest.TransactionFixture()
		err := store.Store(&expected.TransactionBody)
		require.NoError(t, err)

		// retrieve the transaction by ID
		actual, err := store.ByID(expected.ID())
		require.NoError(t, err)
		assert.Equal(t, &expected.TransactionBody, actual)

		// re-insert the transaction - should be idempotent
		err = store.Store(&expected.TransactionBody)
		require.NoError(t, err)
	})
}

func TestTransactionStoreRetrieveByIndex(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewTransactions(metrics, db)
		txID := unittest.TransactionFixture().ID()
		blockID := unittest.BlockFixture().ID()
		index := uint32(0)

		// store a transaction ID in db
		err := store.StoreTxIDByBlockIDTxIndex(blockID, index, txID)
		require.NoError(t, err)

		// retrieve the transaction ID by block and index
		actual, err := store.TransactionIDByBlockIDIndex(blockID, index)
		require.NoError(t, err)
		assert.Equal(t, txID, *actual)

		// re-insert the transaction - should be idempotent
		err = store.StoreTxIDByBlockIDTxIndex(blockID, index, txID)
		require.NoError(t, err)
	})
}

func TestTransactionRetrieveWithoutStore(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewTransactions(metrics, db)

		// attempt to get a invalid transaction
		_, err := store.ByID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})
}
