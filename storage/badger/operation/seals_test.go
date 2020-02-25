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

func TestSealInsertCheckRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.SealFixture()

		err := db.Update(InsertSeal(&expected))
		require.Nil(t, err)

		var exists bool
		err = db.View(CheckSeal(expected.ID(), &exists))
		require.Nil(t, err)
		require.True(t, exists)

		var actual flow.Seal
		err = db.View(RetrieveSeal(expected.ID(), &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestSealIndexAndLookup(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		seal1 := unittest.SealFixture()
		seal2 := unittest.SealFixture()

		seals := []*flow.Seal{&seal1, &seal2}

		payload := flow.MakeID([]byte{0x42})

		expected := make([]flow.Identifier, len(seals))

		err := db.Update(func(tx *badger.Txn) error {
			for i, seal := range seals {
				expected[i] = seal.ID()
				if err := InsertSeal(seal)(tx); err != nil {
					return err
				}
				if err := IndexSeal(payload, uint64(i), seal.ID())(tx); err != nil {
					return err
				}
			}
			return nil
		})
		require.Nil(t, err)

		var actual []flow.Identifier
		err = db.View(LookupSeals(payload, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}
