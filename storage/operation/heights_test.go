package operation_test

import (
	"math/rand"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFinalizedInsertUpdateRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		height := uint64(1337)
		unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertFinalizedHeight(lctx, rw.Writer(), height)
			})
		})

		var retrieved uint64
		err := operation.RetrieveFinalizedHeight(db.Reader(), &retrieved)
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)

		height = 9999
		unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertFinalizedHeight(lctx, rw.Writer(), height)
			})
		})

		err = operation.RetrieveFinalizedHeight(db.Reader(), &retrieved)
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)
	})
}

func TestSealedInsertUpdateRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		height := uint64(1337)
		unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertSealedHeight(lctx, rw.Writer(), height)
			})
		})

		var retrieved uint64
		err := operation.RetrieveSealedHeight(db.Reader(), &retrieved)
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)

		height = 9999
		unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertSealedHeight(lctx, rw.Writer(), height)
			})
		})

		err = operation.RetrieveSealedHeight(db.Reader(), &retrieved)
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)
	})
}

func TestEpochFirstBlockIndex_InsertRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		height := rand.Uint64()
		epoch := rand.Uint64()

		// retrieve when empty errors
		var retrieved uint64
		err := operation.RetrieveEpochFirstHeight(db.Reader(), epoch, &retrieved)
		require.ErrorIs(t, err, storage.ErrNotFound)

		unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			// can insert
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertEpochFirstHeight(lctx, rw, epoch, height)
			})
		})

		// can retrieve
		err = operation.RetrieveEpochFirstHeight(db.Reader(), epoch, &retrieved)
		require.NoError(t, err)
		assert.Equal(t, retrieved, height)

		// retrieve non-existent key errors
		err = operation.RetrieveEpochFirstHeight(db.Reader(), epoch+1, &retrieved)
		require.ErrorIs(t, err, storage.ErrNotFound)

		unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			// insert existent key errors
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertEpochFirstHeight(lctx, rw, epoch, height)
			})
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
			return nil
		})
	})
}
