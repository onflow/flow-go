package operation

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestChunkDataPack(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		collectionID := unittest.IdentifierFixture()
		expected := &storage.StoredChunkDataPack{
			ChunkID:      unittest.IdentifierFixture(),
			StartState:   unittest.StateCommitmentFixture(),
			Proof:        []byte{'p'},
			CollectionID: collectionID,
		}

		t.Run("Retrieve non-existent", func(t *testing.T) {
			var actual storage.StoredChunkDataPack
			err := RetrieveChunkDataPack(expected.ChunkID, &actual)(db)
			assert.Error(t, err)
		})

		t.Run("Save", func(t *testing.T) {
			err := InsertChunkDataPack(expected)(db)
			require.NoError(t, err)

			var actual storage.StoredChunkDataPack
			err = RetrieveChunkDataPack(expected.ChunkID, &actual)(db)
			assert.NoError(t, err)

			assert.Equal(t, *expected, actual)
		})

		t.Run("Remove", func(t *testing.T) {
			err := RemoveChunkDataPack(expected.ChunkID)(db)
			require.NoError(t, err)

			var actual storage.StoredChunkDataPack
			err = RetrieveChunkDataPack(expected.ChunkID, &actual)(db)
			assert.Error(t, err)
		})
	})
}
