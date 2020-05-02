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
		expected := unittest.BlockSealFixture()

		err := db.Update(InsertSeal(expected))
		require.Nil(t, err)

		var exists bool
		err = db.View(CheckSeal(expected.ID(), &exists))
		require.Nil(t, err)
		require.True(t, exists)

		var actual flow.Seal
		err = db.View(RetrieveSeal(expected.ID(), &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)
	})
}

func TestSealIndexAndLookup(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		seal1 := unittest.BlockSealFixture()
		seal2 := unittest.BlockSealFixture()

		seals := []*flow.Seal{seal1, seal2}

		blockID := flow.MakeID([]byte{0x42})

		expected := flow.GetIDs(seals)

		err := db.Update(func(tx *badger.Txn) error {
			for _, seal := range seals {
				if err := InsertSeal(seal)(tx); err != nil {
					return err
				}
			}
			if err := IndexSealPayload(blockID, expected)(tx); err != nil {
				return err
			}
			return nil
		})
		require.Nil(t, err)

		var actual []flow.Identifier
		err = db.View(LookupSealPayload(blockID, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestSealIDByBlockIndexLookup(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		sealID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()

		err := db.Update(IndexSealIDByBlock(blockID, sealID))
		require.Nil(t, err)

		var actual flow.Identifier
		err = db.View(LookupSealIDByBlock(blockID, &actual))
		require.Nil(t, err)

		assert.Equal(t, sealID, actual)
	})
}
