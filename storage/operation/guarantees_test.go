package operation_test

import (
	"testing"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGuaranteeInsertRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		g := unittest.CollectionGuaranteeFixture()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UnsafeInsertGuarantee(rw.Writer(), g.CollectionID, g)
		})
		require.NoError(t, err)

		var retrieved flow.CollectionGuarantee
		err = operation.RetrieveGuarantee(db.Reader(), g.CollectionID, &retrieved)
		require.NoError(t, err)

		assert.Equal(t, g, &retrieved)
	})
}

func TestIndexGuaranteedCollectionByBlockHashInsertRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		blockID := flow.Identifier{0x10}
		collID1 := flow.Identifier{0x01}
		collID2 := flow.Identifier{0x02}
		guarantees := []*flow.CollectionGuarantee{
			{CollectionID: collID1, Signature: crypto.Signature{0x10}},
			{CollectionID: collID2, Signature: crypto.Signature{0x20}},
		}
		expected := flow.GetIDs(guarantees)

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for _, guarantee := range guarantees {
				if err := operation.UnsafeInsertGuarantee(rw.Writer(), guarantee.ID(), guarantee); err != nil {
					return err
				}
			}
			if err := operation.UnsafeIndexPayloadGuarantees(rw.Writer(), blockID, expected); err != nil {
				return err
			}
			return nil
		})
		require.NoError(t, err)

		var actual []flow.Identifier
		err = operation.LookupPayloadGuarantees(db.Reader(), blockID, &actual)
		require.NoError(t, err)

		assert.Equal(t, []flow.Identifier{collID1, collID2}, actual)
	})
}

func TestIndexGuaranteedCollectionByBlockHashMultipleBlocks(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
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
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for _, guarantee := range set1 {
				if err := operation.UnsafeInsertGuarantee(rw.Writer(), guarantee.CollectionID, guarantee); err != nil {
					return err
				}
			}
			if err := operation.UnsafeIndexPayloadGuarantees(rw.Writer(), blockID1, ids1); err != nil {
				return err
			}
			return nil
		})
		require.NoError(t, err)

		// insert block 2
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for _, guarantee := range set2 {
				if err := operation.UnsafeInsertGuarantee(rw.Writer(), guarantee.CollectionID, guarantee); err != nil {
					return err
				}
			}
			if err := operation.UnsafeIndexPayloadGuarantees(rw.Writer(), blockID2, ids2); err != nil {
				return err
			}
			return nil
		})
		require.NoError(t, err)

		t.Run("should retrieve collections for block", func(t *testing.T) {
			var actual1 []flow.Identifier
			err = operation.LookupPayloadGuarantees(db.Reader(), blockID1, &actual1)
			assert.NoError(t, err)
			assert.ElementsMatch(t, []flow.Identifier{collID1}, actual1)

			// get block 2
			var actual2 []flow.Identifier
			err = operation.LookupPayloadGuarantees(db.Reader(), blockID2, &actual2)
			assert.NoError(t, err)
			assert.Equal(t, []flow.Identifier{collID2, collID3, collID4}, actual2)
		})
	})
}
