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
		collections := bstorage.NewCollections(db, transactions)

		// create a light collection with three transactions
		expected := unittest.CollectionFixture(3).Light()

		// store the light collection and the transaction index
		err := collections.StoreLightAndIndexByTransaction(&expected)
		require.Nil(t, err)

		// retrieve the light collection by collection id
		actual, err := collections.LightByID(expected.ID())
		require.Nil(t, err)

		// check if the light collection was indeed persisted
		assert.Equal(t, &expected, actual)

		expectedID := expected.ID()

		// retrieve the collection light id by each of its transaction id
		for _, txID := range expected.Transactions {
			collLight, err := collections.LightByTransactionID(txID)
			actualID := collLight.ID()
			// check that the collection id can indeed be retrieved by transaction id
			require.Nil(t, err)
			assert.Equal(t, expectedID, actualID)
		}

	})
}

func TestCollections_IndexDuplicateTx(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		transactions := bstorage.NewTransactions(metrics, db)
		collections := bstorage.NewCollections(db, transactions)

		// create two collections which share 1 transaction
		col1 := unittest.CollectionFixture(2)
		col2 := unittest.CollectionFixture(1)
		dupTx := col1.Transactions[0]  // the duplicated transaction
		col2Tx := col2.Transactions[0] // transaction that's only in col2
		col2.Transactions = append(col2.Transactions, dupTx)

		// insert col1
		col1Light := col1.Light()
		err := collections.StoreLightAndIndexByTransaction(&col1Light)
		require.NoError(t, err)

		// insert col2
		col2Light := col2.Light()
		err = collections.StoreLightAndIndexByTransaction(&col2Light)
		require.NoError(t, err)

		// should be able to retrieve col2 by ID
		gotLightByCol2ID, err := collections.LightByID(col2.ID())
		require.NoError(t, err)
		assert.Equal(t, &col2Light, gotLightByCol2ID)

		// should be able to retrieve col2 by the transaction which only appears in col2
		_, err = collections.LightByTransactionID(col2Tx.ID())
		require.NoError(t, err)

		// col1 (not col2) should be indexed by the shared transaction (since col1 was inserted first)
		gotLightByDupTxID, err := collections.LightByTransactionID(dupTx.ID())
		require.NoError(t, err)
		assert.Equal(t, &col1Light, gotLightByDupTxID)
	})
}
