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
			payload := flow.MakeID([]byte{0x42})
			coll1 := unittest.CollectionFixture(1).Light()
			coll2 := unittest.CollectionFixture(2).Light()
			colls := []*flow.LightCollection{&coll1, &coll2}

			err := db.Update(func(tx *badger.Txn) error {
				for i, coll := range colls {
					if err := InsertCollection(coll)(tx); err != nil {
						return err
					}
					if err := IndexCollection(payload, uint64(i), coll)(tx); err != nil {
						return err
					}
				}
				return nil
			})
			require.Nil(t, err)

			var actual []flow.Identifier
			err = db.View(LookupCollections(payload, &actual))
			require.Nil(t, err)

			assert.Equal(t, []flow.Identifier{coll1.ID(), coll2.ID()}, actual)
		})
	})
}
