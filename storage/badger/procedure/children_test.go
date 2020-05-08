package procedure_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// after indexing a block by its parent, it should be able to retrieve the child block by the parentID
func TestIndexAndLookupChild(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		parentID := unittest.IdentifierFixture()
		childID := unittest.IdentifierFixture()

		err := db.Update(operation.InsertBlockChildren(parentID, nil))
		require.NoError(t, err)

		err = db.Update(procedure.IndexBlockChild(parentID, childID))
		require.NoError(t, err)

		// retrieve child
		var retrievedIDs []flow.Identifier
		err = db.View(procedure.LookupBlockChildren(parentID, &retrievedIDs))
		require.NoError(t, err)

		// retrieved child should be the stored child
		require.Equal(t, []flow.Identifier{childID}, retrievedIDs)
	})
}

// if two blocks connect to the same parent, indexing the second block would have
// no effect, retrieving the child of the parent block will return the first block that
// was indexed.
func TestIndexTwiceAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		parentID := unittest.IdentifierFixture()
		child1ID := unittest.IdentifierFixture()
		child2ID := unittest.IdentifierFixture()

		err := db.Update(operation.InsertBlockChildren(parentID, nil))
		require.NoError(t, err)

		// index the first child
		err = db.Update(procedure.IndexBlockChild(parentID, child1ID))
		require.NoError(t, err)

		// index the second child
		err = db.Update(procedure.IndexBlockChild(parentID, child2ID))
		require.NoError(t, err)

		// retrieve child
		var retrievedIDs []flow.Identifier
		err = db.View(procedure.LookupBlockChildren(parentID, &retrievedIDs))
		require.NoError(t, err)

		// retrieved child should be the first child
		require.Equal(t, []flow.Identifier{child1ID, child2ID}, retrievedIDs)
	})
}
