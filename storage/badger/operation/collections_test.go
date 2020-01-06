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
		expected := flow.Collection{
			Transactions: []flow.Fingerprint{[]byte{1}, []byte{2}},
		}

		t.Run("Retrieve nonexistant", func(t *testing.T) {
			var actual flow.Collection
			err := db.View(RetrieveCollection(expected.Fingerprint(), &actual))
			assert.Error(t, err)
		})

		t.Run("Save", func(t *testing.T) {
			err := db.Update(InsertCollection(&expected))
			require.NoError(t, err)

			var actual flow.Collection
			err = db.View(RetrieveCollection(expected.Fingerprint(), &actual))
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)
		})

		t.Run("Remove", func(t *testing.T) {
			err := db.Update(RemoveCollection(expected.Fingerprint()))
			require.NoError(t, err)

			var actual flow.Collection
			err = db.View(RetrieveCollection(expected.Fingerprint(), &actual))
			assert.Error(t, err)
		})
	})
}
