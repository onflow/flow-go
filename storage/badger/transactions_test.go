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

func TestTransactionRetrieveWithoutStore(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewTransactions(metrics, db)

		// attempt to get a invalid transaction
		_, err := store.ByID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})
}
