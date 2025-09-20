package operation_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestOperationPair represents a pair of operations to test
type TestOperationPair struct {
	name      string
	indexFunc func(lockctx.Proof, storage.ReaderBatchWriter, flow.Identifier, flow.Identifier) error
	lockType  string
}

// getTestOperationPairs returns the operation pairs to test
func getTestOperationPairs() []TestOperationPair {
	return []TestOperationPair{
		{
			name:      "IndexNewBlock",
			indexFunc: operation.IndexNewBlock,
			lockType:  storage.LockInsertBlock,
		},
		{
			name:      "IndexNewClusterBlock",
			indexFunc: operation.IndexNewClusterBlock,
			lockType:  storage.LockInsertOrFinalizeClusterBlock,
		},
	}
}

// after indexing a block by its parent, it should be able to retrieve the child block by the parentID
func TestIndexAndLookupChild(t *testing.T) {
	for _, opPair := range getTestOperationPairs() {
		t.Run(opPair.name, func(t *testing.T) {
			dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
				lockManager := storage.NewTestingLockManager()
				parentID := unittest.IdentifierFixture()
				childID := unittest.IdentifierFixture()

				unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, childID, parentID)
					})
				})

				// retrieve child
				var retrievedIDs flow.IdentifierList
				require.NoError(t, operation.RetrieveBlockChildren(db.Reader(), parentID, &retrievedIDs))

				// retrieved child should be the stored child
				require.Equal(t, flow.IdentifierList{childID}, retrievedIDs)

				require.NoError(t, operation.RetrieveBlockChildren(db.Reader(), childID, &retrievedIDs))
				// verify new block has no child
				require.Equal(t, flow.IdentifierList(nil), retrievedIDs)

				// verify indexing again would hit storage.ErrAlreadyExists error
				unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, childID, parentID)
					})
					require.Error(t, err)
					require.ErrorIs(t, err, storage.ErrAlreadyExists)
					return nil
				})
			})
		})
	}
}

// if two blocks connect to the same parent, indexing the second block would have
// no effect, retrieving the child of the parent block will return the first block that
// was indexed.
func TestIndexTwiceAndRetrieve(t *testing.T) {
	for _, opPair := range getTestOperationPairs() {
		t.Run(opPair.name, func(t *testing.T) {
			dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
				lockManager := storage.NewTestingLockManager()

				parentID := unittest.IdentifierFixture()
				child1ID := unittest.IdentifierFixture()
				child2ID := unittest.IdentifierFixture()

				// index the first child
				unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, child1ID, parentID)
					})
				})

				// index the second child
				unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, child2ID, parentID)
					})
				})

				var retrievedIDs flow.IdentifierList
				err := operation.RetrieveBlockChildren(db.Reader(), parentID, &retrievedIDs)
				require.NoError(t, err)

				require.Equal(t, flow.IdentifierList{child1ID, child2ID}, retrievedIDs)
			})
		})
	}
}

// if parent is zero, then we don't index it
func TestIndexZeroParent(t *testing.T) {
	for _, opPair := range getTestOperationPairs() {
		t.Run(opPair.name, func(t *testing.T) {
			dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
				lockManager := storage.NewTestingLockManager()

				childID := unittest.IdentifierFixture()

				unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, childID, flow.ZeroID)
					})
				})

				// zero id should have no children
				var retrievedIDs flow.IdentifierList
				err := operation.RetrieveBlockChildren(db.Reader(), flow.ZeroID, &retrievedIDs)
				require.ErrorIs(t, err, storage.ErrNotFound)
			})
		})
	}
}

// lookup block children will only return direct childrens
func TestDirectChildren(t *testing.T) {
	for _, opPair := range getTestOperationPairs() {
		t.Run(opPair.name, func(t *testing.T) {
			dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
				lockManager := storage.NewTestingLockManager()

				b1 := unittest.IdentifierFixture()
				b2 := unittest.IdentifierFixture()
				b3 := unittest.IdentifierFixture()
				b4 := unittest.IdentifierFixture()

				unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, b2, b1)
					})
				})

				unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, b3, b2)
					})
				})

				unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, b4, b3)
					})
				})

				// check the children of the first block
				var retrievedIDs flow.IdentifierList

				err := operation.RetrieveBlockChildren(db.Reader(), b1, &retrievedIDs)
				require.NoError(t, err)
				require.Equal(t, flow.IdentifierList{b2}, retrievedIDs)

				err = operation.RetrieveBlockChildren(db.Reader(), b2, &retrievedIDs)
				require.NoError(t, err)
				require.Equal(t, flow.IdentifierList{b3}, retrievedIDs)

				err = operation.RetrieveBlockChildren(db.Reader(), b3, &retrievedIDs)
				require.NoError(t, err)
				require.Equal(t, flow.IdentifierList{b4}, retrievedIDs)

				err = operation.RetrieveBlockChildren(db.Reader(), b4, &retrievedIDs)
				require.NoError(t, err)
				require.Nil(t, retrievedIDs)
			})
		})
	}
}

// TestChildrenWrongLockIsRejected verifies that operations fail when called with the wrong lock type.
// This ensures that IndexNewBlock requires LockInsertBlock and IndexNewClusterBlock requires LockInsertOrFinalizeClusterBlock.
func TestChildrenWrongLockIsRejected(t *testing.T) {

	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		parentID := unittest.IdentifierFixture()
		childID := unittest.IdentifierFixture()

		unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexNewClusterBlock(lctx, rw, childID, parentID)
			})

			require.Error(t, err)
			return nil
		})

		unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexNewBlock(lctx, rw, childID, parentID)
			})

			require.Error(t, err)
			return nil
		})
	})
}
