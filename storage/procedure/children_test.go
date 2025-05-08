package procedure_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/procedure"
	"github.com/onflow/flow-go/utils/unittest"
)

// after indexing a block by its parent, it should be able to retrieve the child block by the parentID
func TestIndexAndLookupChild(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {

		parentID := unittest.IdentifierFixture()
		childID := unittest.IdentifierFixture()

		indexing := &sync.Mutex{}
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return procedure.IndexNewBlock(indexing, rw, childID, parentID)
		})
		require.NoError(t, err)

		// retrieve child
		var retrievedIDs flow.IdentifierList
		err = procedure.LookupBlockChildren(db.Reader(), parentID, &retrievedIDs)
		require.NoError(t, err)

		// retrieved child should be the stored child
		require.Equal(t, flow.IdentifierList{childID}, retrievedIDs)
	})
}

// if two blocks connect to the same parent, indexing the second block would have
// no effect, retrieving the child of the parent block will return the first block that
// was indexed.
func TestIndexTwiceAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {

		parentID := unittest.IdentifierFixture()
		child1ID := unittest.IdentifierFixture()
		child2ID := unittest.IdentifierFixture()

		indexing := &sync.Mutex{}
		// index the first child
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return procedure.IndexNewBlock(indexing, rw, child1ID, parentID)
		})
		require.NoError(t, err)

		// index the second child
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return procedure.IndexNewBlock(indexing, rw, child2ID, parentID)
		})
		require.NoError(t, err)

		var retrievedIDs flow.IdentifierList
		err = procedure.LookupBlockChildren(db.Reader(), parentID, &retrievedIDs)
		require.NoError(t, err)

		require.Equal(t, flow.IdentifierList{child1ID, child2ID}, retrievedIDs)
	})
}

// if parent is zero, then we don't index it
func TestIndexZeroParent(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {

		childID := unittest.IdentifierFixture()

		indexing := &sync.Mutex{}
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return procedure.IndexNewBlock(indexing, rw, childID, flow.ZeroID)
		})
		require.NoError(t, err)

		// zero id should have no children
		var retrievedIDs flow.IdentifierList
		err = procedure.LookupBlockChildren(db.Reader(), flow.ZeroID, &retrievedIDs)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

// lookup block children will only return direct childrens
func TestDirectChildren(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {

		b1 := unittest.IdentifierFixture()
		b2 := unittest.IdentifierFixture()
		b3 := unittest.IdentifierFixture()
		b4 := unittest.IdentifierFixture()

		indexing := &sync.Mutex{}
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return procedure.IndexNewBlock(indexing, rw, b2, b1)
		})
		require.NoError(t, err)

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return procedure.IndexNewBlock(indexing, rw, b3, b2)
		})
		require.NoError(t, err)

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return procedure.IndexNewBlock(indexing, rw, b4, b3)
		})
		require.NoError(t, err)

		// check the children of the first block
		var retrievedIDs flow.IdentifierList

		err = procedure.LookupBlockChildren(db.Reader(), b1, &retrievedIDs)
		require.NoError(t, err)
		require.Equal(t, flow.IdentifierList{b2}, retrievedIDs)

		err = procedure.LookupBlockChildren(db.Reader(), b2, &retrievedIDs)
		require.NoError(t, err)
		require.Equal(t, flow.IdentifierList{b3}, retrievedIDs)

		err = procedure.LookupBlockChildren(db.Reader(), b3, &retrievedIDs)
		require.NoError(t, err)
		require.Equal(t, flow.IdentifierList{b4}, retrievedIDs)

		err = procedure.LookupBlockChildren(db.Reader(), b4, &retrievedIDs)
		require.NoError(t, err)
		require.Nil(t, retrievedIDs)
	})
}
