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

func TestIdentitiesInsertCheckRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.IdentityFixture()

		err := db.Update(InsertIdentity(expected))
		require.Nil(t, err)

		var exists bool
		err = db.View(CheckIdentity(expected.ID(), &exists))
		require.Nil(t, err)
		require.True(t, exists)

		var actual flow.Identity
		err = db.View(RetrieveIdentity(expected.ID(), &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)
	})
}

func TestIdentitiesIndexAndLookup(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		ids := unittest.IdentityListFixture(4)

		payload := flow.MakeID([]byte{0x42})

		expected := make([]flow.Identifier, len(ids))

		err := db.Update(func(tx *badger.Txn) error {
			for i, id := range ids {
				expected[i] = id.ID()
				if err := InsertIdentity(id)(tx); err != nil {
					return err
				}
				if err := IndexIdentity(payload, uint64(i), id.ID())(tx); err != nil {
					return err
				}
			}
			return nil
		})
		require.Nil(t, err)

		var actual []flow.Identifier
		err = db.View(LookupIdentities(payload, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}
