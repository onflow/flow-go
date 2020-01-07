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
		transactions := badgerstorage.NewTransactions(db)
		collections := badgerstorage.NewCollections(db)

		coll := unittest.CollectionFixture(2)

		t.Run("should be able to save collection", func(t *testing.T) {
			err := collections.Save(&coll)
			assert.NoError(t, err)
		})

		t.Run("should be able to read collection", func(t *testing.T) {
			actual, err := collections.ByFingerprint(coll.Fingerprint())
			assert.NoError(t, err)
			assert.Equal(t, &coll, actual)
		})

		t.Run("should be able to read transactions individually", func(t *testing.T) {
			expected1 := coll.Transactions[0]
			expected2 := coll.Transactions[1]

			actual1, err := transactions.ByFingerprint(expected1.Fingerprint())
			assert.NoError(t, err)
			assert.Equal(t, expected1, actual1.TransactionBody)

			actual2, err := transactions.ByFingerprint(expected2.Fingerprint())
			assert.NoError(t, err)
			assert.Equal(t, expected2, actual2.TransactionBody)
		})
	})
}

// Make sure collection storage still works if transactions have already been
// individually stored.
func TestStoringTransactionsTwice(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		transactions := badgerstorage.NewTransactions(db)
		collections := badgerstorage.NewCollections(db)

		coll := unittest.CollectionFixture(2)
		tx1 := flow.Transaction{TransactionBody: coll.Transactions[0]}
		tx2 := flow.Transaction{TransactionBody: coll.Transactions[1]}

		err := transactions.Insert(&tx1)
		assert.NoError(t, err)
		err = transactions.Insert(&tx2)
		assert.NoError(t, err)

		err = collections.Save(&coll)
		assert.NoError(t, err)
	})
}

func TestRetrievalByNonexistentHash(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		collections := badgerstorage.NewCollections(db)

		collection := unittest.CollectionFixture(4)

		err := collections.Save(&collection)
		require.NoError(t, err)

		_, err = collections.ByFingerprint([]byte("LOL"))

		assert.True(t, errors.Is(err, storage.NotFoundErr))
	})
}

func TestStoringSameCollectionTwice(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		collections := badgerstorage.NewCollections(db)

		collection := unittest.CollectionFixture(3)

		err := collections.Save(&collection)
		require.NoError(t, err)

		err = collections.Save(&collection)
		require.NoError(t, err)
	})
}
