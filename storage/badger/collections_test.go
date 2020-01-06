package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage"
	badger2 "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestCollectionRetrievalByHash(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		collections := badger2.NewCollections(db)

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

		collections := badger2.NewCollections(db)

		collection := unittest.FlowCollectionFixture(4)

		err := collections.Save(&collection)
		require.NoError(t, err)

		_, err = collections.ByFingerprint([]byte("LOL"))

		assert.Equal(t, storage.NotFoundErr, err)
	})
}

func TestStoringSameCollectionTwice(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		collections := badger2.NewCollections(db)

		collection := unittest.FlowCollectionFixture(3)

		err := collections.Save(&collection)
		require.NoError(t, err)

		err = collections.Save(&collection)
		require.NoError(t, err)
	})
}
