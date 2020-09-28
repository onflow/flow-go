package procedure_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/procedure"
	"github.com/onflow/flow-go/utils/unittest"
)

// after indexing a block by its parent, it should be able to retrieve the child block by the parentID
func TestIndexAndLookupChild(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		parentID := unittest.IdentifierFixture()
		childID := unittest.IdentifierFixture()

		err := db.Update(procedure.IndexNewBlock(childID, parentID))
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

		// index the first child
		err := db.Update(procedure.IndexNewBlock(child1ID, parentID))
		require.NoError(t, err)

		// index the second child
		err = db.Update(procedure.IndexNewBlock(child2ID, parentID))
		require.NoError(t, err)

		var retrievedIDs []flow.Identifier
		err = db.View(procedure.LookupBlockChildren(parentID, &retrievedIDs))
		require.NoError(t, err)

		require.Equal(t, []flow.Identifier{child1ID, child2ID}, retrievedIDs)
	})
}

// if parent is zero, then we don't index it
func TestIndexZeroParent(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		childID := unittest.IdentifierFixture()

		err := db.Update(procedure.IndexNewBlock(childID, flow.ZeroID))
		require.NoError(t, err)

		// zero id should have no children
		var retrievedIDs []flow.Identifier
		err = db.View(procedure.LookupBlockChildren(flow.ZeroID, &retrievedIDs))
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

// lookup block children will only return direct childrens
func TestDirectChildren(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		b1 := unittest.IdentifierFixture()
		b2 := unittest.IdentifierFixture()
		b3 := unittest.IdentifierFixture()
		b4 := unittest.IdentifierFixture()

		err := db.Update(procedure.IndexNewBlock(b2, b1))
		require.NoError(t, err)

		err = db.Update(procedure.IndexNewBlock(b3, b2))
		require.NoError(t, err)

		err = db.Update(procedure.IndexNewBlock(b4, b3))
		require.NoError(t, err)

		// check the children of the first block
		var retrievedIDs []flow.Identifier

		err = db.View(procedure.LookupBlockChildren(b1, &retrievedIDs))
		require.NoError(t, err)
		require.Equal(t, []flow.Identifier{b2}, retrievedIDs)

		err = db.View(procedure.LookupBlockChildren(b2, &retrievedIDs))
		require.NoError(t, err)
		require.Equal(t, []flow.Identifier{b3}, retrievedIDs)

		err = db.View(procedure.LookupBlockChildren(b3, &retrievedIDs))
		require.NoError(t, err)
		require.Equal(t, []flow.Identifier{b4}, retrievedIDs)

		err = db.View(procedure.LookupBlockChildren(b4, &retrievedIDs))
		require.NoError(t, err)
		require.Nil(t, retrievedIDs)
	})
}
