package pebble

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestChunksQueue(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		q := NewChunkQueue(db)
		inited, err := q.Init(0)
		require.NoError(t, err)
		require.True(t, inited)

		// veirfy the index is inited as 0
		index, err := q.LatestIndex()
		require.NoError(t, err)
		require.Equal(t, uint64(0), index)

		locator := &chunks.Locator{
			ResultID: unittest.IdentifierFixture(),
			Index:    1,
		}

		// store a locator
		stored, err := q.StoreChunkLocator(locator)
		require.NoError(t, err)
		require.True(t, stored)

		// verify the index is incremented
		index, err = q.LatestIndex()
		require.NoError(t, err)
		require.Equal(t, uint64(1), index)

		// verify is able to read the locator
		loc, err := q.AtIndex(index)
		require.NoError(t, err)
		require.Equal(t, locator, loc)

		// verify un-indexed locator
		_, err = q.AtIndex(index + 1)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// verify the latest index is persisted
		q2 := NewChunkQueue(db)
		inited, err = q2.Init(0)
		require.NoError(t, err)
		require.False(t, inited)

		index2, err := q2.LatestIndex()
		require.NoError(t, err)
		require.Equal(t, uint64(1), index2)
	})
}
