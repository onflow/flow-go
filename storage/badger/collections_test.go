package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	badgerstorage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGetCollectionTransactions(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		txStore := badgerstorage.NewTransactions(db)
		collStore := badgerstorage.NewCollections(db)

		tx1 := unittest.TransactionFixture(func(t *flow.Transaction) {
			t.Nonce = 1
		})
		tx2 := unittest.TransactionFixture(func(t *flow.Transaction) {
			t.Nonce = 2
		})
		collection := flow.Collection{
			Transactions: []flow.Fingerprint{tx1.Fingerprint(), tx2.Fingerprint()},
		}

		err := txStore.Insert(&tx1)
		assert.NoError(t, err)
		err = txStore.Insert(&tx2)
		assert.NoError(t, err)
		err = collStore.Save(&collection)
		assert.NoError(t, err)

		actual, err := collStore.ByFingerprint(collection.Fingerprint())
		assert.NoError(t, err)
		assert.Equal(t, &collection, actual)

		transactions, err := collStore.TransactionsByFingerprint(collection.Fingerprint())
		assert.NoError(t, err)
		assert.Len(t, transactions, 2)
		assert.Equal(t, tx1, *transactions[0])
		assert.Equal(t, tx2, *transactions[1])
	})
}

func TestCollectionRetrievalByHash(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		collections := badgerstorage.NewCollections(db)

		collection := unittest.FlowCollectionFixture(2)

		err := collections.Save(&collection)
		require.NoError(t, err)

		byHash, err := collections.ByFingerprint(collection.Fingerprint())
		require.NoError(t, err)

		assert.Equal(t, collection, *byHash)
	})
}

func TestRetrievalByNonexistentHash(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		collections := badgerstorage.NewCollections(db)

		collection := unittest.FlowCollectionFixture(4)

		err := collections.Save(&collection)
		require.NoError(t, err)

		_, err = collections.ByFingerprint([]byte("LOL"))

		assert.True(t, errors.Is(err, storage.NotFoundErr))
	})
}

func TestStoringSameCollectionTwice(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		collections := badgerstorage.NewCollections(db)

		collection := unittest.FlowCollectionFixture(3)

		err := collections.Save(&collection)
		require.NoError(t, err)

		err = collections.Save(&collection)
		require.NoError(t, err)
	})
}
