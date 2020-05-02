package procedure_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// after indexing a block by its parent, it should be able to retrieve the child block by the parentID
func TestIndexAndLookupChild(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		block1 := &flow.Header{
			ParentID: flow.ZeroID,
			View:     1,
		}
		block2 := &flow.Header{
			ParentID: block1.ID(),
			View:     2,
		}

		// store parent
		headersDB := storage.NewHeaders(db)
		err := headersDB.Store(block1)
		require.NoError(t, err)

		// stored headers can be retrieved
		var childHeader flow.Header
		err = db.View(operation.RetrieveHeader(block1.ID(), &childHeader))
		require.NoError(t, err)

		// store child
		err = headersDB.Store(block2)
		require.NoError(t, err)

		parentID, childID := block1.ID(), block2.ID()

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
		block1 := &flow.Header{
			ParentID: flow.ZeroID,
			View:     1,
		}
		// block2 and block3 are children of block1
		block2 := &flow.Header{
			ParentID: block1.ID(),
			View:     2,
		}
		block3 := &flow.Header{
			ParentID: block1.ID(),
			View:     3,
		}

		// store parent
		headersDB := storage.NewHeaders(db)
		err := headersDB.Store(block1)
		require.NoError(t, err)

		// store child
		err = headersDB.Store(block2)
		require.NoError(t, err)

		// store another child
		err = headersDB.Store(block3)
		require.NoError(t, err)

		parentID, childID, secondChildID := block1.ID(), block2.ID(), block3.ID()

		// index the first child
		err = db.Update(procedure.IndexBlockChild(parentID, childID))
		require.NoError(t, err)

		// index the second child
		err = db.Update(procedure.IndexBlockChild(parentID, secondChildID))
		require.NoError(t, err)

		// retrieve child
		var retrievedIDs []flow.Identifier
		err = db.View(procedure.LookupBlockChildren(parentID, &retrievedIDs))
		require.NoError(t, err)

		// retrieved child should be the first child
		require.Equal(t, []flow.Identifier{childID, secondChildID}, retrievedIDs)
	})
}
