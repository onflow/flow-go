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

		t.Run("Check", func(t *testing.T) {
			var exists bool
			err := db.Update(CheckCollection(expected.ID(), &exists))
			require.NoError(t, err)
			assert.True(t, exists)
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
	})
}
