package pebble

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// 1. should be able to read the latest index after store
func TestChunksQueueInitAndReadLatest(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		queue := NewChunkQueue(db)
		inited, err := queue.Init(10)
		require.NoError(t, err)
		require.Equal(t, true, inited)
		latest, err := queue.LatestIndex()
		require.NoError(t, err)
		require.Equal(t, uint64(10), latest)
	})
}

func makeLocator() *chunks.Locator {
	return &chunks.Locator{
		ResultID: unittest.IdentifierFixture(),
		Index:    0,
	}
}

// 2. should be able to read after store
func TestChunksQueueStoreAndRead(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		queue := NewChunkQueue(db)
		_, err := queue.Init(0)
		require.NoError(t, err)

		locator := makeLocator()
		stored, err := queue.StoreChunkLocator(locator)
		require.NoError(t, err)
		require.True(t, stored)

		latest, err := queue.LatestIndex()
		require.NoError(t, err)
		require.Equal(t, uint64(1), latest)

		latestJob, err := queue.AtIndex(latest)
		require.NoError(t, err)
		require.Equal(t, locator, latestJob)

		// can read again
		latestJob, err = queue.AtIndex(latest)
		require.NoError(t, err)
		require.Equal(t, locator, latestJob)

		// store the same locator again
		stored, err = queue.StoreChunkLocator(locator)
		require.NoError(t, err)
		require.False(t, stored)

		// non existing job
		_, err = queue.AtIndex(latest + 1)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestChunksQueueStoreMulti(t *testing.T) {

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		queue := NewChunkQueue(db)
		_, err := queue.Init(0)
		require.NoError(t, err)

		locators := make([]*chunks.Locator, 0, 100)
		for i := 0; i < 100; i++ {
			locators = append(locators, makeLocator())
		}

		// store and read
		for i := 0; i < 10; i++ {
			stored, err := queue.StoreChunkLocator(locators[i])
			require.NoError(t, err)
			require.True(t, stored)

			latest, err := queue.LatestIndex()
			require.NoError(t, err)
			require.Equal(t, uint64(i+1), latest)
		}

		// store then read
		for i := 0; i < 10; i++ {

			latestJob, err := queue.AtIndex(uint64(i + 1))
			require.NoError(t, err)
			require.Equal(t, locators[i], latestJob)
		}

		for i := 10; i < 100; i++ {
			stored, err := queue.StoreChunkLocator(locators[i])
			require.NoError(t, err)
			require.True(t, stored)

			latest, err := queue.LatestIndex()
			require.NoError(t, err)
			require.Equal(t, uint64(i+1), latest)
		}

		for i := 10; i < 100; i++ {
			latestJob, err := queue.AtIndex(uint64(i + 1))
			require.NoError(t, err)
			require.Equal(t, locators[i], latestJob)
		}
	})
}
