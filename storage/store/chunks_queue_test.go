package store

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// 1. should be able to read after store
// 2. should be able to read the latest index after store
// 3. should return false if a duplicate chunk is stored
// 4. should return true if a new chunk is stored
// 5. should return an increased index when a chunk is stored
// 6. storing 100 chunks concurrent should return last index as 100
// 7. should not be able to read with wrong index
// 8. should return init index after init
// 9. storing chunk and updating the latest index should be atomic
func TestChunksQueue(t *testing.T) {
	t.Run("store and read", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			q := NewChunkQueue(db)
			locators := unittest.ChunkLocatorListFixture(1)
			locator := locators[0]

			// Store the locator
			stored, err := q.StoreChunkLocator(locator)
			require.NoError(t, err)
			require.True(t, stored)

			// Read the locator
			retrieved, err := q.AtIndex(1)
			require.NoError(t, err)
			require.Equal(t, locator, retrieved)
		})
	})

	t.Run("latest index after store", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			q := NewChunkQueue(db)
			locators := unittest.ChunkLocatorListFixture(1)
			locator := locators[0]

			_, err := q.StoreChunkLocator(locator)
			require.NoError(t, err)

			latest, err := q.LatestIndex()
			require.NoError(t, err)
			require.Equal(t, uint64(1), latest)
		})
	})

	t.Run("duplicate chunk storage", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			q := NewChunkQueue(db)
			locators := unittest.ChunkLocatorListFixture(1)
			locator := locators[0]

			stored, err := q.StoreChunkLocator(locator)
			require.NoError(t, err)
			require.True(t, stored)

			stored, err = q.StoreChunkLocator(locator)
			require.NoError(t, err)
			require.False(t, stored)
		})
	})

	t.Run("increasing index", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			q := NewChunkQueue(db)
			locators := unittest.ChunkLocatorListFixture(1)
			locator1, locator2 := locators[0], locators[1]

			_, err := q.StoreChunkLocator(locator1)
			require.NoError(t, err)

			_, err = q.StoreChunkLocator(locator2)
			require.NoError(t, err)

			latest, err := q.LatestIndex()
			require.NoError(t, err)
			require.Equal(t, uint64(2), latest)
		})
	})

	t.Run("concurrent storage", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			q := NewChunkQueue(db)
			locators := unittest.ChunkLocatorListFixture(100)

			for _, locator := range locators {
				_, err := q.StoreChunkLocator(locator)
				require.NoError(t, err)
			}

			latest, err := q.LatestIndex()
			require.NoError(t, err)
			require.Equal(t, uint64(len(locators)), latest)
		})
	})

	t.Run("read with wrong index", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			q := NewChunkQueue(db)

			_, err := q.AtIndex(1)
			require.Error(t, err)
		})
	})

	t.Run("init index", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			q := NewChunkQueue(db)
			defaultIndex := uint64(10)

			initialized, err := q.Init(defaultIndex)
			require.NoError(t, err)
			require.True(t, initialized)

			latest, err := q.LatestIndex()
			require.NoError(t, err)
			require.Equal(t, defaultIndex, latest)

			// Trying to init again should return false
			initialized, err = q.Init(defaultIndex)
			require.NoError(t, err)
			require.False(t, initialized)
		})
	})
}
