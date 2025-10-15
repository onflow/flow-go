package operation_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestChunkDataPack tests basic operation for storing ChunkDataPacks themselves.
// The data is typically stored in the chunk data packs database.
func TestChunkDataPack(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		collectionID := unittest.IdentifierFixture()
		expected := &storage.StoredChunkDataPack{
			ChunkID:      unittest.IdentifierFixture(),
			StartState:   unittest.StateCommitmentFixture(),
			Proof:        []byte{'p'},
			CollectionID: collectionID,
		}

		t.Run("Retrieve non-existent", func(t *testing.T) {
			var actual storage.StoredChunkDataPack
			err := operation.RetrieveChunkDataPack(db.Reader(), expected.ID(), &actual)
			assert.Error(t, err)
		})

		t.Run("Save", func(t *testing.T) {
			require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertChunkDataPack(rw, expected.ID(), expected)
			}))

			var actual storage.StoredChunkDataPack
			err := operation.RetrieveChunkDataPack(db.Reader(), expected.ID(), &actual)
			assert.NoError(t, err)

			assert.Equal(t, *expected, actual)
		})

		t.Run("Remove", func(t *testing.T) {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.RemoveChunkDataPack(rw.Writer(), expected.ID())
			})
			require.NoError(t, err)

			var actual storage.StoredChunkDataPack
			err = operation.RetrieveChunkDataPack(db.Reader(), expected.ID(), &actual)
			assert.Error(t, err)
		})
	})
}

// TestIndexChunkDataPackByChunkID tests populating the index mapping from chunkID to the resulting chunk data pack's ID.
// Essentially, the chunk ID describes the work to be done, and the chunk data pack describes the result of that work.
// The index from chunk ID to chunk data pack ID is typically stored in the protocol database.
func TestIndexChunkDataPackByChunkID(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		chunkID := unittest.IdentifierFixture()
		chunkDataPackID := unittest.IdentifierFixture()

		t.Run("successful insert", func(t *testing.T) {
			err := unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexChunkDataPackByChunkID(lctx, rw, chunkID, chunkDataPackID)
				})
			})
			require.NoError(t, err)

			// Verify the chunk data pack ID was indexed
			var retrievedID flow.Identifier
			err = operation.RetrieveChunkDataPackID(db.Reader(), chunkID, &retrievedID)
			require.NoError(t, err)
			assert.Equal(t, chunkDataPackID, retrievedID)
		})

		t.Run("idempotent insert", func(t *testing.T) {
			// Insert the same chunk data pack ID again should be idempotent
			err := unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexChunkDataPackByChunkID(lctx, rw, chunkID, chunkDataPackID)
				})
			})
			require.NoError(t, err)

			// Verify the chunk data pack ID is still the same
			var retrievedID flow.Identifier
			err = operation.RetrieveChunkDataPackID(db.Reader(), chunkID, &retrievedID)
			require.NoError(t, err)
			assert.Equal(t, chunkDataPackID, retrievedID)
		})

		t.Run("data mismatch error", func(t *testing.T) {
			differentStoredChunkDataPackID := unittest.IdentifierFixture()

			err := unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexChunkDataPackByChunkID(lctx, rw, chunkID, differentStoredChunkDataPackID)
				})
			})
			require.Error(t, err)
			assert.ErrorIs(t, err, storage.ErrDataMismatch)
			assert.Contains(t, err.Error(), "different one exist")

			// Verify the chunk data pack ID is unchanged
			var retrievedID flow.Identifier
			err = operation.RetrieveChunkDataPackID(db.Reader(), chunkID, &retrievedID)
			require.NoError(t, err)
			assert.Equal(t, chunkDataPackID, retrievedID)
		})

		t.Run("missing required lock", func(t *testing.T) {
			newChunkID := unittest.IdentifierFixture()
			newStoredChunkDataPackID := unittest.IdentifierFixture()

			// Create a context without any locks
			lctx := lockManager.NewContext()
			defer lctx.Release()

			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexChunkDataPackByChunkID(lctx, rw, newChunkID, newStoredChunkDataPackID)
			})
			require.Error(t, err)
			require.NotErrorIs(t, err, storage.ErrDataMismatch) // missing lock should not be erroneously represented as mismatching data
			assert.Contains(t, err.Error(), "missing required lock")
			assert.Contains(t, err.Error(), storage.LockInsertOwnReceipt)
		})

		t.Run("wrong lock type", func(t *testing.T) {
			newChunkID := unittest.IdentifierFixture()
			newStoredChunkDataPackID := unittest.IdentifierFixture()

			// Use wrong lock type
			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexChunkDataPackByChunkID(lctx, rw, newChunkID, newStoredChunkDataPackID)
				})
			})
			require.Error(t, err)
			require.NotErrorIs(t, err, storage.ErrDataMismatch) // wrong lock should not be erroneously represented as mismatching data
			assert.Contains(t, err.Error(), "missing required lock")
			assert.Contains(t, err.Error(), storage.LockInsertOwnReceipt)
		})
	})
}

// TestRetrieveChunkDataPackID tests reading the index mapping from chunkID to the resulting chunk data pack's ID.
// Essentially, the chunk ID describes the work to be done, and the chunk data pack describes the result of that work.
// The index from chunk ID to chunk data pack ID is typically stored in the protocol database.
func TestRetrieveChunkDataPackID(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		chunkID := unittest.IdentifierFixture()
		chunkDataPackID := unittest.IdentifierFixture()

		t.Run("retrieve non-existent", func(t *testing.T) {
			var retrievedID flow.Identifier
			err := operation.RetrieveChunkDataPackID(db.Reader(), chunkID, &retrievedID)
			require.Error(t, err)
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("retrieve existing", func(t *testing.T) {
			// First insert a chunk data pack ID
			err := unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexChunkDataPackByChunkID(lctx, rw, chunkID, chunkDataPackID)
				})
			})
			require.NoError(t, err)

			// Then retrieve it
			var retrievedID flow.Identifier
			err = operation.RetrieveChunkDataPackID(db.Reader(), chunkID, &retrievedID)
			require.NoError(t, err)
			assert.Equal(t, chunkDataPackID, retrievedID)
		})

		t.Run("retrieve after removal", func(t *testing.T) {
			// Insert a chunk data pack ID
			err := unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexChunkDataPackByChunkID(lctx, rw, chunkID, chunkDataPackID)
				})
			})
			require.NoError(t, err)

			// Remove it
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.RemoveChunkDataPackID(rw.Writer(), chunkID)
			})
			require.NoError(t, err)

			// Try to retrieve it again - should fail
			var retrievedID flow.Identifier
			err = operation.RetrieveChunkDataPackID(db.Reader(), chunkID, &retrievedID)
			require.Error(t, err)
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})
	})
}

// TestRemoveChunkDataPackID tests the RemoveChunkDataPackID operation
func TestRemoveChunkDataPackID(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		chunkID := unittest.IdentifierFixture()
		chunkDataPackID := unittest.IdentifierFixture()

		t.Run("remove non-existent", func(t *testing.T) {
			// Removing a non-existent chunk data pack ID should not error
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.RemoveChunkDataPackID(rw.Writer(), chunkID)
			})
			require.NoError(t, err)
		})

		t.Run("remove existing", func(t *testing.T) {
			// First insert a chunk data pack ID
			err := unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexChunkDataPackByChunkID(lctx, rw, chunkID, chunkDataPackID)
				})
			})
			require.NoError(t, err)

			// Verify it exists
			var retrievedID flow.Identifier
			err = operation.RetrieveChunkDataPackID(db.Reader(), chunkID, &retrievedID)
			require.NoError(t, err)
			assert.Equal(t, chunkDataPackID, retrievedID)

			// Remove it
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.RemoveChunkDataPackID(rw.Writer(), chunkID)
			})
			require.NoError(t, err)

			// Verify it's gone
			err = operation.RetrieveChunkDataPackID(db.Reader(), chunkID, &retrievedID)
			require.Error(t, err)
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("remove multiple", func(t *testing.T) {
			chunkIDs := unittest.IdentifierListFixture(3)
			chunkDataPackIDs := unittest.IdentifierListFixture(3)

			// Insert multiple chunk data pack IDs
			for i := 0; i < 3; i++ {

				err := unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return operation.IndexChunkDataPackByChunkID(lctx, rw, chunkIDs[i], chunkDataPackIDs[i])
					})
				})
				require.NoError(t, err)
			}

			// Remove all of them
			for i := 0; i < 3; i++ {
				err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.RemoveChunkDataPackID(rw.Writer(), chunkIDs[i])
				})
				require.NoError(t, err)
			}

			// Verify all are gone
			for i := 0; i < 3; i++ {
				var retrievedID flow.Identifier
				err := operation.RetrieveChunkDataPackID(db.Reader(), chunkIDs[i], &retrievedID)
				require.Error(t, err)
				assert.ErrorIs(t, err, storage.ErrNotFound)
			}
		})
	})
}
