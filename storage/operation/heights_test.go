package operation_test

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
)

func TestFinalizedInsertUpdateRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		height := uint64(1337)

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertFinalizedHeight(rw.Writer(), height)
		})
		require.NoError(t, err)

		var retrieved uint64
		err = operation.RetrieveFinalizedHeight(db.Reader(), &retrieved)
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)

		height = 9999
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertFinalizedHeight(rw.Writer(), height)
		})
		require.NoError(t, err)

		err = operation.RetrieveFinalizedHeight(db.Reader(), &retrieved)
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)
	})
}

func TestSealedInsertUpdateRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		height := uint64(1337)

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertSealedHeight(rw.Writer(), height)
		})
		require.NoError(t, err)

		var retrieved uint64
		err = operation.RetrieveSealedHeight(db.Reader(), &retrieved)
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)

		height = 9999
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertSealedHeight(rw.Writer(), height)
		})
		require.NoError(t, err)

		err = operation.RetrieveSealedHeight(db.Reader(), &retrieved)
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)
	})
}

func TestEpochFirstBlockIndex_InsertRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		height := rand.Uint64()
		epoch := rand.Uint64()

		// retrieve when empty errors
		var retrieved uint64
		err := operation.RetrieveEpochFirstHeight(db.Reader(), epoch, &retrieved)
		require.ErrorIs(t, err, storage.ErrNotFound)

		inserting := &sync.Mutex{}
		// can insert
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertEpochFirstHeight(inserting, rw, epoch, height)
		})
		require.NoError(t, err)

		// can retrieve
		err = operation.RetrieveEpochFirstHeight(db.Reader(), epoch, &retrieved)
		require.NoError(t, err)
		assert.Equal(t, retrieved, height)

		// retrieve non-existent key errors
		err = operation.RetrieveEpochFirstHeight(db.Reader(), epoch+1, &retrieved)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// insert existent key errors
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertEpochFirstHeight(inserting, rw, epoch, height)
		})
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
	})
}

func TestLastCompleteBlockHeightInsertUpdateRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		height := uint64(1337)

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertLastCompleteBlockHeight(rw.Writer(), height)
		})
		require.NoError(t, err)

		var retrieved uint64
		err = operation.RetrieveLastCompleteBlockHeight(db.Reader(), &retrieved)
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)

		height = 9999
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertLastCompleteBlockHeight(rw.Writer(), height)
		})
		require.NoError(t, err)

		err = operation.RetrieveLastCompleteBlockHeight(db.Reader(), &retrieved)
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)
	})
}

func TestLastCompleteBlockHeightInsertIfNotExists(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		height1 := uint64(1337)

		inserting := &sync.Mutex{}
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertLastCompleteBlockHeightIfNotExists(inserting, rw, height1)
		})
		require.NoError(t, err)

		var retrieved uint64
		err = operation.RetrieveLastCompleteBlockHeight(db.Reader(), &retrieved)
		require.NoError(t, err)

		assert.Equal(t, retrieved, height1)

		height2 := uint64(9999)
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertLastCompleteBlockHeightIfNotExists(inserting, rw, height2)
		})
		require.NoError(t, err)

		err = operation.RetrieveLastCompleteBlockHeight(db.Reader(), &retrieved)
		require.NoError(t, err)

		assert.Equal(t, retrieved, height1)
	})
}
