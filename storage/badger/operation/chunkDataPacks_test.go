package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestChunkDataPack(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.ChunkDataPackFixture(unittest.IdentifierFixture())

		t.Run("Retrieve non-existent", func(t *testing.T) {
			var actual flow.ChunkDataPack
			err := db.View(RetrieveChunkDataPack(expected.ChunkID, &actual))
			assert.Error(t, err)
		})

		t.Run("Save", func(t *testing.T) {
			err := db.Update(InsertChunkDataPack(&expected))
			require.NoError(t, err)

			var actual flow.ChunkDataPack
			err = db.View(RetrieveChunkDataPack(expected.ChunkID, &actual))
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)
		})

		t.Run("Remove", func(t *testing.T) {
			err := db.Update(RemoveChunkDataPack(expected.ChunkID))
			require.NoError(t, err)

			var actual flow.ChunkDataPack
			err = db.View(RetrieveChunkDataPack(expected.ChunkID, &actual))
			assert.Error(t, err)
		})
	})
}
