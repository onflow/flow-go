package operation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

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
			err := operation.RetrieveChunkDataPack(db.Reader(), expected.ChunkID, &actual)
			assert.Error(t, err)
		})

		t.Run("Save", func(t *testing.T) {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertChunkDataPack(rw.Writer(), expected)
			})
			require.NoError(t, err)

			var actual storage.StoredChunkDataPack
			err = operation.RetrieveChunkDataPack(db.Reader(), expected.ChunkID, &actual)
			assert.NoError(t, err)

			assert.Equal(t, *expected, actual)
		})

		t.Run("Remove", func(t *testing.T) {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.RemoveChunkDataPack(rw.Writer(), expected.ChunkID)
			})
			require.NoError(t, err)

			var actual storage.StoredChunkDataPack
			err = operation.RetrieveChunkDataPack(db.Reader(), expected.ChunkID, &actual)
			assert.Error(t, err)
		})
	})
}
