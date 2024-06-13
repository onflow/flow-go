package procedure_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/procedure"
	"github.com/onflow/flow-go/utils/unittest"
)

// after indexing a block by its parent, it should be able to retrieve the child block by the parentID
func TestIndexAndLookupChild(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		parentID := unittest.IdentifierFixture()
		childID := unittest.IdentifierFixture()

		err := procedure.IndexNewBlock(childID, parentID)(db)
		require.NoError(t, err)

		// retrieve child
		var retrievedIDs flow.IdentifierList
		err = procedure.LookupBlockChildren(parentID, &retrievedIDs)(db)
		require.NoError(t, err)

		// retrieved child should be the stored child
		require.Equal(t, flow.IdentifierList{childID}, retrievedIDs)
	})
}

// if two blocks connect to the same parent, indexing the second block would have
// no effect, retrieving the child of the parent block will return the first block that
// was indexed.
func TestIndexTwiceAndRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		parentID := unittest.IdentifierFixture()
		child1ID := unittest.IdentifierFixture()
		child2ID := unittest.IdentifierFixture()

		// index the first child
		err := procedure.IndexNewBlock(child1ID, parentID)(db)
		require.NoError(t, err)

		// index the second child
		err = procedure.IndexNewBlock(child2ID, parentID)(db)
		require.NoError(t, err)

		var retrievedIDs flow.IdentifierList
		err = procedure.LookupBlockChildren(parentID, &retrievedIDs)(db)
		require.NoError(t, err)

		require.Equal(t, flow.IdentifierList{child1ID, child2ID}, retrievedIDs)
	})
}

// if parent is zero, then we don't index it
func TestIndexZeroParent(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		childID := unittest.IdentifierFixture()

		err := procedure.IndexNewBlock(childID, flow.ZeroID)(db)
		require.NoError(t, err)

		// zero id should have no children
		var retrievedIDs flow.IdentifierList
		err = procedure.LookupBlockChildren(flow.ZeroID, &retrievedIDs)(db)
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

// lookup block children will only return direct childrens
func TestDirectChildren(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		b1 := unittest.IdentifierFixture()
		b2 := unittest.IdentifierFixture()
		b3 := unittest.IdentifierFixture()
		b4 := unittest.IdentifierFixture()

		err := procedure.IndexNewBlock(b2, b1)(db)
		require.NoError(t, err)

		err = procedure.IndexNewBlock(b3, b2)(db)
		require.NoError(t, err)

		err = procedure.IndexNewBlock(b4, b3)(db)
		require.NoError(t, err)

		// check the children of the first block
		var retrievedIDs flow.IdentifierList

		err = procedure.LookupBlockChildren(b1, &retrievedIDs)(db)
		require.NoError(t, err)
		require.Equal(t, flow.IdentifierList{b2}, retrievedIDs)

		err = procedure.LookupBlockChildren(b2, &retrievedIDs)(db)
		require.NoError(t, err)
		require.Equal(t, flow.IdentifierList{b3}, retrievedIDs)

		err = procedure.LookupBlockChildren(b3, &retrievedIDs)(db)
		require.NoError(t, err)
		require.Equal(t, flow.IdentifierList{b4}, retrievedIDs)

		fmt.Println("lookup")
		err = procedure.LookupBlockChildren(b4, &retrievedIDs)(db)
		require.NoError(t, err)
		require.Nil(t, retrievedIDs)
	})
}
