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

func TestGuaranteeInsertCheckRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		g := unittest.CollectionGuaranteeFixture()

		err := db.Update(InsertGuarantee(g))
		require.Nil(t, err)

		var exists bool
		err = db.View(CheckGuarantee(g.CollectionID, &exists))
		require.NoError(t, err)
		require.True(t, exists)

		var retrieved flow.CollectionGuarantee
		err = db.View(RetrieveGuarantee(g.CollectionID, &retrieved))
		require.NoError(t, err)

		assert.Equal(t, g, &retrieved)
	})
}

func TestIndexGuaranteedCollectionByBlockHashInsertRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		blockID := flow.Identifier{0x10}
		collID1 := flow.Identifier{0x01}
		collID2 := flow.Identifier{0x02}
		collections := []*flow.CollectionGuarantee{
			{CollectionID: collID1, Signature: crypto.Signature{0x10}},
			{CollectionID: collID2, Signature: crypto.Signature{0x20}},
		}
		expected := flow.GetIDs(collections)

		err := db.Update(func(tx *badger.Txn) error {
			for _, coll := range collections {
				if err := InsertGuarantee(coll)(tx); err != nil {
					return err
				}
			}
			if err := IndexGuaranteePayload(blockID, expected)(tx); err != nil {
				return err
			}
			return nil
		})
		require.Nil(t, err)

		var actual []flow.Identifier
		err = db.View(LookupGuaranteePayload(blockID, &actual))
		require.Nil(t, err)

		assert.Equal(t, []flow.Identifier{collID1, collID2}, actual)
	})
}

func TestIndexGuaranteedCollectionByBlockHashMultipleBlocks(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		blockID1 := flow.Identifier{0x10}
		blockID2 := flow.Identifier{0x20}
		collID1 := flow.Identifier{0x01}
		collID2 := flow.Identifier{0x02}
		collID3 := flow.Identifier{0x03}
		collID4 := flow.Identifier{0x04}
		set1 := []*flow.CollectionGuarantee{
			{CollectionID: collID1, Signature: crypto.Signature{0x1}},
		}
		set2 := []*flow.CollectionGuarantee{
			{CollectionID: collID2, Signature: crypto.Signature{0x2}},
			{CollectionID: collID3, Signature: crypto.Signature{0x3}},
			{CollectionID: collID4, Signature: crypto.Signature{0x1}},
		}
		ids1 := flow.GetIDs(set1)
		ids2 := flow.GetIDs(set2)

		// insert block 1
		err := db.Update(func(tx *badger.Txn) error {
			for _, coll := range set1 {
				if err := InsertGuarantee(coll)(tx); err != nil {
					return err
				}
			}
			if err := IndexGuaranteePayload(blockID1, ids1)(tx); err != nil {
				return err
			}
			return nil
		})
		require.Nil(t, err)

		// insert block 2
		err = db.Update(func(tx *badger.Txn) error {
			for _, coll := range set2 {
				if err := InsertGuarantee(coll)(tx); err != nil {
					return err
				}
			}
			if err := IndexGuaranteePayload(blockID2, ids2)(tx); err != nil {
				return err
			}
			return nil
		})
		require.Nil(t, err)

		t.Run("should retrieve collections for block", func(t *testing.T) {
			var actual1 []flow.Identifier
			err = db.View(LookupGuaranteePayload(blockID1, &actual1))
			assert.NoError(t, err)
			assert.ElementsMatch(t, []flow.Identifier{collID1}, actual1)

			// get block 2
			var actual2 []flow.Identifier
			err = db.View(LookupGuaranteePayload(blockID2, &actual2))
			assert.NoError(t, err)
			assert.Equal(t, []flow.Identifier{collID2, collID3, collID4}, actual2)
		})
	})
}
