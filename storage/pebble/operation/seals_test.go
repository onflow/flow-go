package operation

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSealInsertCheckRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		expected := unittest.Seal.Fixture()

		err := InsertSeal(expected.ID(), expected)(db)
		require.Nil(t, err)

		var actual flow.Seal
		err = RetrieveSeal(expected.ID(), &actual)(db)
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)
	})
}

func TestSealIndexAndLookup(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		seal1 := unittest.Seal.Fixture()
		seal2 := unittest.Seal.Fixture()

		seals := []*flow.Seal{seal1, seal2}

		blockID := flow.MakeID([]byte{0x42})

		expected := []flow.Identifier(flow.GetIDs(seals))

		batch := db.NewBatch()

		err := func(tx pebble.Writer) error {
			for _, seal := range seals {
				if err := InsertSeal(seal.ID(), seal)(tx); err != nil {
					return err
				}
			}
			if err := IndexPayloadSeals(blockID, expected)(tx); err != nil {
				return err
			}

			return batch.Commit(nil)
		}(batch)
		require.Nil(t, err)

		var actual []flow.Identifier
		err = LookupPayloadSeals(blockID, &actual)(db)
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}
