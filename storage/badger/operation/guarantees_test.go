package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestIndexGuaranteedCollectionByBlockHashInsertRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		blockID := flow.Identifier{0x12, 0x34}
		expected := []*flow.CollectionGuarantee{
			{CollectionID: flow.Identifier{0x01}, Signatures: []crypto.Signature{{0x10}}},
			{CollectionID: flow.Identifier{0x02}, Signatures: []crypto.Signature{{0x20}}},
		}

		err := db.Update(func(tx *badger.Txn) error {
			for _, coll := range expected {
				if err := InsertGuarantee(coll)(tx); err != nil {
					return err
				}
				if err := IndexGuarantee(blockID, coll)(tx); err != nil {
					return err
				}
			}
			return nil
		})
		require.Nil(t, err)

		var actual []*flow.CollectionGuarantee
		err = db.View(RetrieveGuarantees(blockID, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestIndexGuaranteedCollectionByBlockHashMultipleBlocks(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		blockID1 := flow.Identifier{0x10}
		blockID2 := flow.Identifier{0x20}
		expected1 := []*flow.CollectionGuarantee{
			{CollectionID: flow.Identifier{0x1}, Signatures: []crypto.Signature{{0x1}}},
		}
		expected2 := []*flow.CollectionGuarantee{
			{CollectionID: flow.Identifier{0x2}, Signatures: []crypto.Signature{{0x2}}},
		}

		// insert block 1
		err := db.Update(func(tx *badger.Txn) error {
			for _, coll := range expected1 {
				if err := InsertGuarantee(coll)(tx); err != nil {
					return err
				}
				if err := IndexGuarantee(blockID1, coll)(tx); err != nil {
					return err
				}
			}
			return nil
		})
		require.Nil(t, err)

		// insert block 2
		err = db.Update(func(tx *badger.Txn) error {
			for _, coll := range expected2 {
				if err := InsertGuarantee(coll)(tx); err != nil {
					return err
				}
				if err := IndexGuarantee(blockID2, coll)(tx); err != nil {
					return err
				}
			}
			return nil
		})
		require.Nil(t, err)

		t.Run("should retrieve collections for block", func(t *testing.T) {
			var actual1 []*flow.CollectionGuarantee
			err = db.View(RetrieveGuarantees(blockID1, &actual1))
			assert.NoError(t, err)
			assert.Equal(t, expected1, actual1)

			// get block 2
			var actual2 []*flow.CollectionGuarantee
			err = db.View(RetrieveGuarantees(blockID2, &actual2))
			assert.NoError(t, err)
			assert.Equal(t, expected1, actual1)
		})
	})
}
