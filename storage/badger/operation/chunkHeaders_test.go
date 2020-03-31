package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestChunkHeaders(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(t *testing.T, db *badger.DB) {
		expected := unittest.ChunkHeaderFixture()

		t.Run("Retrieve non-existent", func(t *testing.T) {
			var actual flow.ChunkHeader
			err := db.View(RetrieveChunkHeader(expected.ChunkID, &actual))
			assert.Error(t, err)
		})

		t.Run("Save", func(t *testing.T) {
			err := db.Update(InsertChunkHeader(&expected))
			require.NoError(t, err)

			var actual flow.ChunkHeader
			err = db.View(RetrieveChunkHeader(expected.ChunkID, &actual))
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)
		})

		t.Run("Remove", func(t *testing.T) {
			err := db.Update(RemoveChunkHeader(expected.ChunkID))
			require.NoError(t, err)

			var actual flow.ChunkHeader
			err = db.View(RetrieveChunkHeader(expected.ChunkID, &actual))
			assert.Error(t, err)
		})
	})
}
