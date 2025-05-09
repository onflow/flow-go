package store_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactionStoreRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := store.NewTransactions(metrics, db)

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
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := store.NewTransactions(metrics, db)

		// attempt to get a invalid transaction
		_, err := store.ByID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

func TestTransactionRemove(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := store.NewTransactions(metrics, db)

		// Create and store a transaction
		expected := unittest.TransactionFixture()
		err := store.Store(&expected.TransactionBody)
		require.NoError(t, err)

		// Ensure it exists
		tx, err := store.ByID(expected.ID())
		require.NoError(t, err)
		assert.Equal(t, &expected.TransactionBody, tx)

		// Remove it
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return store.RemoveBatch(rw, expected.ID())
		})
		require.NoError(t, err)

		// Ensure it no longer exists
		_, err = store.ByID(expected.ID())
		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})
}
