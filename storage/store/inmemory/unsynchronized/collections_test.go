package unsynchronized

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCollection_HappyCase(t *testing.T) {
	transactions := NewTransactions()
	collections := NewCollections(transactions)

	collection := unittest.CollectionFixture(3)

	// Store collection
	_, err := collections.Store(&collection)
	require.NoError(t, err)

	// Retrieve collection
	retrieved, err := collections.ByID(collection.ID())
	require.NoError(t, err)
	require.Equal(t, &collection, retrieved)

	// Extract collections
	extracted := collections.Data()
	require.Len(t, extracted, 1)
	require.Equal(t, collection, extracted[0])

	// Remove collection
	err = collections.Remove(collection.ID())
	require.NoError(t, err)

	retrieved, err = collections.ByID(collection.ID())
	require.ErrorIs(t, err, storage.ErrNotFound)
	require.Nil(t, retrieved)
}

func TestLightByTransactionID_HappyCase(t *testing.T) {
	transactions := NewTransactions()
	collections := NewCollections(transactions)
	collection := unittest.CollectionFixture(2)

	// Create a no-op lock context for testing
	_, err := collections.StoreAndIndexByTransaction(nil, &collection)
	require.NoError(t, err)

	// Fetch by transaction ID and validate
	retrieved, err := collections.LightByTransactionID(collection.Transactions[0].ID())
	require.NoError(t, err)
	lightCollection := collection.Light()
	require.Equal(t, *lightCollection, *retrieved)

	retrieved, err = collections.LightByTransactionID(collection.Transactions[1].ID())
	require.NoError(t, err)
	require.Equal(t, *lightCollection, *retrieved)

	extracted := collections.LightCollections()
	require.Len(t, extracted, 1)
	require.Equal(t, *lightCollection, extracted[0])
}
