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

// TestOperationPair is a pair of indexing operation and corresponding lock to be held during the operation.
// The name is an auxiliary identifier, for debugging only.
type TestOperationPair struct {
	name      string
	indexFunc func(lockctx.Proof, storage.ReaderBatchWriter, flow.Identifier, flow.Identifier) error
	lockType  string
}

// getTestOperationPairs returns a `TestOperationPair` for indexing main consensus blocks
// and one `TestOperationPair` for indexing collector blocks (aka collections).
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
				nonExist := unittest.IdentifierFixture()

				// retrieving children of a non-existent block should return empty list
				var retrievedIDs flow.IdentifierList
				err := operation.RetrieveBlockChildren(db.Reader(), nonExist, &retrievedIDs)
				require.ErrorIs(t, err, storage.ErrNotFound)

				parentID := unittest.IdentifierFixture()
				childID := unittest.IdentifierFixture()

				err = unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, childID, parentID)
					})
				})
				require.NoError(t, err)

				// retrieve child
				require.NoError(t, operation.RetrieveBlockChildren(db.Reader(), parentID, &retrievedIDs))

				// retrieved child should be the stored child
				require.Equal(t, flow.IdentifierList{childID}, retrievedIDs)

				err = operation.RetrieveBlockChildren(db.Reader(), childID, &retrievedIDs)
				// verify new block has no children index (returning storage.ErrNotFound)
				require.ErrorIs(t, err, storage.ErrNotFound)

				// verify indexing again would hit storage.ErrAlreadyExists error
				err = unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, childID, parentID)
					})
				})
				require.ErrorIs(t, err, storage.ErrAlreadyExists)
			})
		})
	}
}

// indexing multiple children to the same parent should be retrievable
func TestIndexWithMultiChildrenRetrieve(t *testing.T) {
	for _, opPair := range getTestOperationPairs() {
		t.Run(opPair.name, func(t *testing.T) {
			dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
				lockManager := storage.NewTestingLockManager()

				parentID := unittest.IdentifierFixture()
				child1ID := unittest.IdentifierFixture()
				child2ID := unittest.IdentifierFixture()

				// index the first child
				err := unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, child1ID, parentID)
					})
				})
				require.NoError(t, err)

				// index the second child
				err = unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, child2ID, parentID)
					})
				})
				require.NoError(t, err)

				var retrievedIDs flow.IdentifierList
				err = operation.RetrieveBlockChildren(db.Reader(), parentID, &retrievedIDs)
				require.NoError(t, err)

				require.ElementsMatch(t, flow.IdentifierList{child1ID, child2ID}, retrievedIDs)
			})
		})
	}
}

// Test indexing the same child with different parents should not error
func TestIndexAgainWithDifferentParentShouldError(t *testing.T) {
	for _, opPair := range getTestOperationPairs() {
		t.Run(opPair.name, func(t *testing.T) {
			dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
				lockManager := storage.NewTestingLockManager()

				child := unittest.IdentifierFixture()
				parent1 := unittest.IdentifierFixture()
				parent2 := unittest.IdentifierFixture()

				// index with parent
				err := unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, child, parent1)
					})
				})
				require.NoError(t, err)

				// index with a different parent
				err = unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, child, parent2)
					})
				})
				require.NoError(t, err)

				var retrievedIDs flow.IdentifierList
				err = operation.RetrieveBlockChildren(db.Reader(), parent1, &retrievedIDs)
				require.NoError(t, err)

				require.ElementsMatch(t, flow.IdentifierList{child}, retrievedIDs)

				err = operation.RetrieveBlockChildren(db.Reader(), parent2, &retrievedIDs)
				require.NoError(t, err)
				require.ElementsMatch(t, flow.IdentifierList{child}, retrievedIDs)
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

				err := unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, childID, flow.ZeroID)
					})
				})
				require.NoError(t, err)

				// zero id should have no children
				var retrievedIDs flow.IdentifierList
				err = operation.RetrieveBlockChildren(db.Reader(), flow.ZeroID, &retrievedIDs)
				require.Error(t, err)
				require.ErrorIs(t, err, storage.ErrNotFound)
			})
		})
	}
}

// lookup block children will only return direct children, even if grandchildren exist
func TestDirectChildren(t *testing.T) {
	for _, opPair := range getTestOperationPairs() {
		t.Run(opPair.name, func(t *testing.T) {
			dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
				lockManager := storage.NewTestingLockManager()

				// We emulate a fork of blocks: b1 ← b2 ← b3 ← b4
				b1 := unittest.IdentifierFixture()
				b2 := unittest.IdentifierFixture()
				b3 := unittest.IdentifierFixture()
				b4 := unittest.IdentifierFixture()

				err := unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, b2, b1)
					})
				})
				require.NoError(t, err)

				err = unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, b3, b2)
					})
				})
				require.NoError(t, err)

				err = unittest.WithLock(t, lockManager, opPair.lockType, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return opPair.indexFunc(lctx, rw, b4, b3)
					})
				})
				require.NoError(t, err)

				// check the children of the first block
				var retrievedIDs flow.IdentifierList

				err = operation.RetrieveBlockChildren(db.Reader(), b1, &retrievedIDs)
				require.NoError(t, err)
				require.Equal(t, flow.IdentifierList{b2}, retrievedIDs)

				err = operation.RetrieveBlockChildren(db.Reader(), b2, &retrievedIDs)
				require.NoError(t, err)
				require.Equal(t, flow.IdentifierList{b3}, retrievedIDs)

				err = operation.RetrieveBlockChildren(db.Reader(), b3, &retrievedIDs)
				require.NoError(t, err)
				require.Equal(t, flow.IdentifierList{b4}, retrievedIDs)

				err = operation.RetrieveBlockChildren(db.Reader(), b4, &retrievedIDs)
				// verify b4 has no children index (not indexed yet)
				require.ErrorIs(t, err, storage.ErrNotFound)
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

		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexNewClusterBlock(lctx, rw, childID, parentID)
			})
		})
		require.Error(t, err)

		err = unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexNewBlock(lctx, rw, childID, parentID)
			})
		})
		require.Error(t, err)
	})
}
