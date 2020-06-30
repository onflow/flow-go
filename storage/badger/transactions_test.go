package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/module/metrics"
	bstorage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TestTransactions tests that a transaction can be stored, retrieved and attempted to be stored again without an error
func TestTransactions(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewTransactions(metrics, db)

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
