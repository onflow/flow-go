package unsynchronized

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCollection_HappyCase(t *testing.T) {
	collections := NewCollections(unittest.Logger())

	collection := unittest.CollectionFixture(3)

	// Store collection
	err := collections.Store(&collection)
	require.NoError(t, err)

	// Retrieve collection
	retrieved, err := collections.ByID(collection.ID())
	require.NoError(t, err)
	require.Equal(t, &collection, retrieved)

	// Remove collection
	err = collections.Remove(collection.ID())
	require.NoError(t, err)

	retrieved, err = collections.ByID(collection.ID())
	require.ErrorIs(t, err, storage.ErrNotFound)
	require.Nil(t, retrieved)
}

func TestLightByTransactionID_HappyCase(t *testing.T) {
	collections := NewCollections(unittest.Logger())
	lightCollection := &flow.LightCollection{
		Transactions: []flow.Identifier{unittest.IdentifierFixture(), unittest.IdentifierFixture()},
	}

	err := collections.StoreLightAndIndexByTransaction(lightCollection)
	require.NoError(t, err)

	// Fetch by transaction ID and validate
	retrieved, err := collections.LightByTransactionID(lightCollection.Transactions[0])
	require.NoError(t, err)
	require.Equal(t, lightCollection, retrieved)

	retrieved, err = collections.LightByTransactionID(lightCollection.Transactions[1])
	require.NoError(t, err)
	require.Equal(t, lightCollection, retrieved)
}

func TestCollection_Persist(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		collections := NewCollections(unittest.Logger())
		collection := unittest.CollectionFixture(3)

		// Store collection
		err := collections.Store(&collection)
		require.NoError(t, err)
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return collections.AddToBatch(rw)
		}))

		// Get light transaction
		var actualCollection flow.LightCollection
		err = operation.RetrieveCollection(db.Reader(), collection.ID(), &actualCollection)
		require.NoError(t, err)
		require.Equal(t, collection.Light(), actualCollection)
	})
}
