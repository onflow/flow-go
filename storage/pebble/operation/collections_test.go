package operation

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCollections(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		expected := unittest.CollectionFixture(2).Light()

		t.Run("Retrieve nonexistant", func(t *testing.T) {
			var actual flow.LightCollection
			err := RetrieveCollection(expected.ID(), &actual)(db)
			assert.Error(t, err)
		})

		t.Run("Save", func(t *testing.T) {
			err := InsertCollection(&expected)(db)
			require.NoError(t, err)

			var actual flow.LightCollection
			err = RetrieveCollection(expected.ID(), &actual)(db)
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)
		})

		t.Run("Remove", func(t *testing.T) {
			err := RemoveCollection(expected.ID())(db)
			require.NoError(t, err)

			var actual flow.LightCollection
			err = RetrieveCollection(expected.ID(), &actual)(db)
			assert.Error(t, err)
		})

		t.Run("Index and lookup", func(t *testing.T) {
			expected := unittest.CollectionFixture(1).Light()
			blockID := unittest.IdentifierFixture()

			_ = BatchUpdate(db, func(tx PebbleReaderWriter) error {
				err := InsertCollection(&expected)(tx)
				assert.Nil(t, err)
				err = IndexCollectionPayload(blockID, expected.Transactions)(tx)
				assert.Nil(t, err)
				return nil
			})

			var actual flow.LightCollection
			err := LookupCollectionPayload(blockID, &actual.Transactions)(db)
			assert.Nil(t, err)

			assert.Equal(t, expected, actual)
		})

		t.Run("Index and lookup by transaction ID", func(t *testing.T) {
			expected := unittest.IdentifierFixture()
			transactionID := unittest.IdentifierFixture()
			actual := flow.Identifier{}

			err := IndexCollectionByTransaction(transactionID, expected)(db)
			assert.Nil(t, err)
			err = RetrieveCollectionID(transactionID, &actual)(db)
			assert.Nil(t, err)
			assert.Equal(t, expected, actual)
		})
	})
}
