package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	storagemodel "github.com/onflow/flow-go/storage/badger/model"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunkDataPack(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := badgerstorage.NewChunkDataPacks(&metrics.NoopCollector{}, db, 100)

		// attempt to get an invalid
		_, err := store.ByChunkID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store in db
		chunkID := unittest.IdentifierFixture()
		expected := unittest.ChunkDataPackFixture(chunkID)
		err = store.Store(expected)
		require.NoError(t, err)

		// retrieve the transaction by ID
		actual, err := store.ByChunkID(chunkID)
		require.NoError(t, err)

		// Note that ChunkDataPack is stored and retrieved as StoredChunkDataPack.
		collID := expected.Collection.ID()
		expectedStored := &storagemodel.StoredChunkDataPack{
			ChunkID:      expected.ChunkID,
			StartState:   expected.StartState,
			Proof:        expected.Proof,
			CollectionID: &collID,
		}
		assert.Equal(t, expectedStored, actual)

		// re-insert - should be idempotent
		err = store.Store(expected)
		require.NoError(t, err)
	})
}
