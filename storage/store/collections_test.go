package store_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCollections(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {

		metrics := metrics.NewNoopCollector()
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)

		// create a light collection with three transactions
		expected := unittest.CollectionFixture(3).Light()

		// store the light collection and the transaction index
		err := collections.StoreLightAndIndexByTransaction(&expected)
		require.NoError(t, err)

		// retrieve the light collection by collection id
		actual, err := collections.LightByID(expected.ID())
		require.NoError(t, err)

		// check if the light collection was indeed persisted
		assert.Equal(t, &expected, actual)

		expectedID := expected.ID()

		// retrieve the collection light id by each of its transaction id
		for _, txID := range expected.Transactions {
			collLight, err := collections.LightByTransactionID(txID)
			actualID := collLight.ID()
			// check that the collection id can indeed be retrieved by transaction id
			require.NoError(t, err)
			assert.Equal(t, expectedID, actualID)
		}

		// remove the collection
		require.NoError(t, collections.Remove(expected.ID()))

		// check that the collection was indeed removed
		_, err = collections.LightByID(expected.ID())
		assert.Error(t, err)
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// check that the collection was indeed removed from the transaction index
		for _, tx := range expected.Transactions {
			_, err = collections.LightByTransactionID(tx)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, storage.ErrNotFound))
		}
	})
}

func TestCollections_IndexDuplicateTx(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)

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

		// col2 (not col1) should be indexed by the shared transaction (since col1 was overwritten by col2)
		gotLightByDupTxID, err := collections.LightByTransactionID(dupTx.ID())
		require.NoError(t, err)
		assert.Equal(t, &col1Light, gotLightByDupTxID)
	})
}

// verify that when StoreLightAndIndexByTransaction is concurrently called with same tx and
// different collection both will succeed, and one of the collection will be indexed by the tx
func TestCollections_ConcurrentIndexByTx(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)

		// Create two collections sharing the same transaction
		const numCollections = 100

		// Create collections sharing the same transaction
		col1 := unittest.CollectionFixture(1)
		col2 := unittest.CollectionFixture(1)
		sharedTx := col1.Transactions[0] // The shared transaction
		col2.Transactions[0] = sharedTx

		var wg sync.WaitGroup
		errChan := make(chan error, 2*numCollections)

		// Insert col1 batch
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < numCollections; i++ {
				col := unittest.CollectionFixture(1)
				col.Transactions[0] = sharedTx // Ensure it shares the same transaction
				light := col.Light()
				err := collections.StoreLightAndIndexByTransaction(&light)
				errChan <- err
			}
		}()

		// Insert col2 batch
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < numCollections; i++ {
				col := unittest.CollectionFixture(1)
				col.Transactions[0] = sharedTx // Ensure it shares the same transaction
				light := col.Light()
				err := collections.StoreLightAndIndexByTransaction(&light)
				errChan <- err
			}
		}()

		wg.Wait()
		close(errChan)

		// Ensure all operations succeeded
		for err := range errChan {
			require.NoError(t, err)
		}

		// Verify that one of the collections is indexed by the shared transaction
		indexedCollection, err := collections.LightByTransactionID(sharedTx.ID())
		require.NoError(t, err)
		assert.True(t, indexedCollection.ID() == col1.ID() || indexedCollection.ID() == col2.ID(), "Expected one of the collections to be indexed")
	})
}
