package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCollections(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		metrics := metrics.NewNoopCollector()
		transactions := bstorage.NewTransactions(metrics, db)
		store := bstorage.NewCollections(db, transactions)

		// create a light collection with three transactions
		expected := unittest.CollectionFixture(3).Light()

		// store the light collection and the transaction index
		err := store.StoreLightAndIndexByTransaction(&expected)
		require.Nil(t, err)

		// retrieve the light collection by collection id
		actual, err := store.LightByID(expected.ID())
		require.Nil(t, err)

		// check if the light collection was indeed persisted
		assert.Equal(t, &expected, actual)

		expectedID := expected.ID()

		// retrieve the collection light id by each of its transaction id
		for _, txID := range expected.Transactions {
			collLight, err := store.LightByTransactionID(txID)
			actualID := collLight.ID()
			// check that the collection id can indeed be retrieved by transaction id
			require.Nil(t, err)
			assert.Equal(t, expectedID, actualID)
		}

	})
}
