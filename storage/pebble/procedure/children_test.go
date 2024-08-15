package procedure_test

import (
	"errors"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/storage/pebble/procedure"
	"github.com/onflow/flow-go/utils/unittest"
)

// after indexing a block by its parent, it should be able to retrieve the child block by the parentID
func TestIndexAndLookupChild(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		indexer := procedure.NewBlockIndexer()

		parentID := unittest.IdentifierFixture()
		childID := unittest.IdentifierFixture()

		rw := operation.NewPebbleReaderBatchWriter(db)
		err := indexer.IndexNewBlock(childID, parentID)(rw)
		require.NoError(t, err)
		require.NoError(t, rw.Commit())

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
		indexer := procedure.NewBlockIndexer()

		parentID := unittest.IdentifierFixture()
		child1ID := unittest.IdentifierFixture()
		child2ID := unittest.IdentifierFixture()

		rw := operation.NewPebbleReaderBatchWriter(db)
		// index the first child
		err := indexer.IndexNewBlock(child1ID, parentID)(rw)
		require.NoError(t, err)
		require.NoError(t, rw.Commit())

		// index the second child
		rw = operation.NewPebbleReaderBatchWriter(db)
		err = indexer.IndexNewBlock(child2ID, parentID)(rw)
		require.NoError(t, err)
		require.NoError(t, rw.Commit())

		var retrievedIDs flow.IdentifierList
		err = procedure.LookupBlockChildren(parentID, &retrievedIDs)(db)
		require.NoError(t, err)

		require.Equal(t, flow.IdentifierList{child1ID, child2ID}, retrievedIDs)
	})
}

// if parent is zero, then we don't index it
func TestIndexZeroParent(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		indexer := procedure.NewBlockIndexer()

		childID := unittest.IdentifierFixture()

		rw := operation.NewPebbleReaderBatchWriter(db)
		err := indexer.IndexNewBlock(childID, flow.ZeroID)(rw)
		require.NoError(t, err)
		require.NoError(t, rw.Commit())

		// zero id should have no children
		var retrievedIDs flow.IdentifierList
		err = procedure.LookupBlockChildren(flow.ZeroID, &retrievedIDs)(db)
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

// lookup block children will only return direct childrens
func TestDirectChildren(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		indexer := procedure.NewBlockIndexer()

		b1 := unittest.IdentifierFixture()
		b2 := unittest.IdentifierFixture()
		b3 := unittest.IdentifierFixture()
		b4 := unittest.IdentifierFixture()

		rw := operation.NewPebbleReaderBatchWriter(db)
		err := indexer.IndexNewBlock(b2, b1)(rw)
		require.NoError(t, err)
		require.NoError(t, rw.Commit())

		rw = operation.NewPebbleReaderBatchWriter(db)
		err = indexer.IndexNewBlock(b3, b2)(rw)
		require.NoError(t, err)
		require.NoError(t, rw.Commit())

		rw = operation.NewPebbleReaderBatchWriter(db)
		err = indexer.IndexNewBlock(b4, b3)(rw)
		require.NoError(t, err)
		require.NoError(t, rw.Commit())

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

		err = procedure.LookupBlockChildren(b4, &retrievedIDs)(db)
		require.NoError(t, err)
		require.Nil(t, retrievedIDs)
	})
}
