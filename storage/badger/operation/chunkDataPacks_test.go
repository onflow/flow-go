package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	storagemodel "github.com/onflow/flow-go/storage/badger/model"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestChunkDataPack(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		collectionID := unittest.IdentifierFixture()
		expected := &storagemodel.StoredChunkDataPack{
			ChunkID:      unittest.IdentifierFixture(),
			StartState:   unittest.StateCommitmentFixture(),
			Proof:        []byte{'p'},
			CollectionID: collectionID,
		}

		t.Run("Retrieve non-existent", func(t *testing.T) {
			var actual storagemodel.StoredChunkDataPack
			err := db.View(RetrieveChunkDataPack(expected.ChunkID, &actual))
			assert.Error(t, err)
		})

		t.Run("Save", func(t *testing.T) {
			err := db.Update(InsertChunkDataPack(expected))
			require.NoError(t, err)

			var actual storagemodel.StoredChunkDataPack
			err = db.View(RetrieveChunkDataPack(expected.ChunkID, &actual))
			assert.NoError(t, err)

			assert.Equal(t, *expected, actual)
		})

		t.Run("Remove", func(t *testing.T) {
			err := db.Update(RemoveChunkDataPack(expected.ChunkID))
			require.NoError(t, err)

			var actual storagemodel.StoredChunkDataPack
			err = db.View(RetrieveChunkDataPack(expected.ChunkID, &actual))
			assert.Error(t, err)
		})
	})
}
