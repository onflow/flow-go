// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestCollections(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.CollectionFixture(2).Light()

		t.Run("Retrieve nonexistant", func(t *testing.T) {
			var actual flow.LightCollection
			err := db.View(RetrieveCollection(expected.ID(), &actual))
			assert.Error(t, err)
		})

		t.Run("Save", func(t *testing.T) {
			err := db.Update(InsertCollection(&expected))
			require.NoError(t, err)

			var actual flow.LightCollection
			err = db.View(RetrieveCollection(expected.ID(), &actual))
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)
		})

		t.Run("Remove", func(t *testing.T) {
			err := db.Update(RemoveCollection(expected.ID()))
			require.NoError(t, err)

			var actual flow.LightCollection
			err = db.View(RetrieveCollection(expected.ID(), &actual))
			assert.Error(t, err)
		})

		t.Run("Index and lookup", func(t *testing.T) {
			expected := unittest.CollectionFixture(1).Light()
			blockID := unittest.IdentifierFixture()

			_ = db.Update(func(tx *badger.Txn) error {
				err := InsertCollection(&expected)(tx)
				assert.Nil(t, err)
				err = IndexCollectionPayload(blockID, expected.Transactions)(tx)
				assert.Nil(t, err)
				return nil
			})

			var actual flow.LightCollection
			err := db.View(LookupCollectionPayload(blockID, &actual.Transactions))
			assert.Nil(t, err)

			assert.Equal(t, expected, actual)
		})

		t.Run("Index and lookup by transaction ID", func(t *testing.T) {
			expected := unittest.IdentifierFixture()
			transactionID := unittest.IdentifierFixture()
			actual := flow.Identifier{}

			_ = db.Update(func(tx *badger.Txn) error {
				err := IndexCollectionByTransaction(transactionID, expected)(tx)
				assert.Nil(t, err)
				err = RetrieveCollectionID(transactionID, &actual)(tx)
				assert.Nil(t, err)
				return nil
			})
			assert.Equal(t, expected, actual)
		})
		t.Run("FilterByNonExistingIDs", func(t *testing.T) {

			// create and persist 2 collections
			persistedColl1 := unittest.CollectionFixture(2).Light()
			persistedColl2 := unittest.CollectionFixture(2).Light()

			err := db.Update(InsertCollection(&persistedColl1))
			require.NoError(t, err)
			err = db.Update(InsertCollection(&persistedColl2))
			require.NoError(t, err)

			// create but do not persist 2 more collections
			notPersistedColl1 := unittest.CollectionFixture(2).Light()
			notPersistedColl2 := unittest.CollectionFixture(2).Light()

			// create the input map
			checkIDMap := make(map[flow.Identifier]struct{})
			checkIDMap[persistedColl1.ID()] = struct{}{}
			checkIDMap[persistedColl2.ID()] = struct{}{}
			checkIDMap[notPersistedColl1.ID()] = struct{}{}
			checkIDMap[notPersistedColl2.ID()] = struct{}{}

			// call FilterByNonExistingIDs
			err = db.View(FilterByNonExistingIDs(checkIDMap))
			assert.NoError(t, err)

			// assert that the map now only has the two non-persisted collection ids
			assert.Len(t, checkIDMap, 2)
			assert.Contains(t, checkIDMap, notPersistedColl1.ID())
			assert.Contains(t, checkIDMap, notPersistedColl2.ID())
		})
	})
}
