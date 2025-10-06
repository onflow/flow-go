package inmemory

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCollection_HappyCase(t *testing.T) {
	collection := unittest.CollectionFixture(3)
	lightCollection := collection.Light()
	collectionID := collection.ID()

	collections := NewCollections([]*flow.Collection{&collection})

	// Retrieve collection
	retrieved, err := collections.ByID(collectionID)
	require.NoError(t, err)
	require.Equal(t, &collection, retrieved)

	retrievedLight, err := collections.LightByTransactionID(collection.Transactions[1].ID())
	require.NoError(t, err)
	require.Equal(t, *lightCollection, *retrievedLight)

	retrievedLight, err = collections.LightByID(collectionID)
	require.NoError(t, err)
	require.Equal(t, *lightCollection, *retrievedLight)

	retrieved, err = collections.ByID(unittest.IdentifierFixture())
	require.ErrorIs(t, err, storage.ErrNotFound)
	require.Nil(t, retrieved)

	retrievedLight, err = collections.LightByTransactionID(unittest.IdentifierFixture())
	require.ErrorIs(t, err, storage.ErrNotFound)
	require.Nil(t, retrievedLight)

	retrievedLight, err = collections.LightByID(unittest.IdentifierFixture())
	require.ErrorIs(t, err, storage.ErrNotFound)
	require.Nil(t, retrievedLight)
}
