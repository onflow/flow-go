package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSealInsertCheckRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.Seal.Fixture()

		err := db.Update(InsertSeal(expected.ID(), expected))
		require.Nil(t, err)

		var actual flow.Seal
		err = db.View(RetrieveSeal(expected.ID(), &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)
	})
}

func TestSealIndexAndLookup(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		seal1 := unittest.Seal.Fixture()
		seal2 := unittest.Seal.Fixture()

		seals := []*flow.Seal{seal1, seal2}

		blockID := flow.MakeID([]byte{0x42})

		expected := []flow.Identifier(flow.GetIDs(seals))

		err := db.Update(func(tx *badger.Txn) error {
			for _, seal := range seals {
				if err := InsertSeal(seal.ID(), seal)(tx); err != nil {
					return err
				}
			}
			if err := IndexPayloadSeals(blockID, expected)(tx); err != nil {
				return err
			}
			return nil
		})
		require.Nil(t, err)

		var actual []flow.Identifier
		err = db.View(LookupPayloadSeals(blockID, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}
