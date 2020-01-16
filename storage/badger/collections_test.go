package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	bstorage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGetCollectionTransactions(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		transactions := bstorage.NewTransactions(db)
		collections := bstorage.NewCollections(db)

		coll := unittest.CollectionFixture(2)

		t.Run("should be able to save collection", func(t *testing.T) {
			err := collections.Store(&coll)
			assert.NoError(t, err)
		})

		t.Run("should be able to read collection", func(t *testing.T) {
			actual, err := collections.ByID(coll.ID())
			assert.NoError(t, err)
			assert.Equal(t, &coll, actual)
		})

		t.Run("should be able to read transactions individually", func(t *testing.T) {
			expected1 := coll.Transactions[0]
			expected2 := coll.Transactions[1]

			actual1, err := transactions.ByID(expected1.ID())
			assert.NoError(t, err)
			assert.Equal(t, expected1, actual1)

			actual2, err := transactions.ByID(expected2.ID())
			assert.NoError(t, err)
			assert.Equal(t, expected2, actual2)
		})
	})
}

// Make sure collection storage still works if transactions have already been
// individually stored.
func TestStoringTransactionsTwice(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		transactions := bstorage.NewTransactions(db)
		collections := bstorage.NewCollections(db)

		coll := unittest.CollectionFixture(2)
		tx1 := coll.Transactions[0]
		tx2 := coll.Transactions[1]

		err := transactions.Store(tx1)
		assert.NoError(t, err)
		err = transactions.Store(tx2)
		assert.NoError(t, err)

		err = collections.Store(&coll)
		assert.NoError(t, err)
	})
}

func TestRetrievalByNonexistentHash(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		collections := bstorage.NewCollections(db)

		collection := unittest.CollectionFixture(4)

		err := collections.Store(&collection)
		require.NoError(t, err)

		_, err = collections.ByID(flow.HashToID([]byte("LOL")))

		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

func TestStoringSameCollectionTwice(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		collections := bstorage.NewCollections(db)

		collection := unittest.CollectionFixture(3)

		err := collections.Store(&collection)
		require.NoError(t, err)

		err = collections.Store(&collection)
		require.NoError(t, err)
	})
}
