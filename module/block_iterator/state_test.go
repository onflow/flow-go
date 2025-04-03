package block_iterator

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProgress(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		root := uint64(10)

		latest := uint64(20)
		getLatest := func() (uint64, error) {
			return latest, nil
		}

		initializer := store.NewConsumerProgress(pebbleimpl.ToDB(db), "test")

		progress, err := NewPersistentIteratorState(initializer, root, getLatest)
		require.NoError(t, err)

		// 1. verify initial state should be the next of root
		next, err := progress.LoadState()
		require.NoError(t, err)
		require.Equal(t, root+1, next)

		rg, hasNext, err := progress.NextRange()
		require.NoError(t, err)
		require.True(t, hasNext)
		require.Equal(t, root+1, rg.Start)
		require.Equal(t, latest, rg.End)

		// save the state
		err = progress.SaveState(latest + 1)
		require.NoError(t, err)

		// 2. verify the saved state
		next, err = progress.LoadState()
		require.NoError(t, err)
		require.Equal(t, latest+1, next)

		// 3. verify when latest is updated to a higher height
		// 		the end height of the next range should be updated
		oldLatest := latest
		latest = latest + 20
		rg, hasNext, err = progress.NextRange()
		require.NoError(t, err)
		require.True(t, hasNext)

		// verify the new range
		require.Equal(t, oldLatest+1, rg.Start)
		require.Equal(t, latest, rg.End)

		// 4. verify when state is up to date, and latest
		// 		does not change, the next range should include no block
		err = progress.SaveState(latest + 1)
		require.NoError(t, err)

		// verify that NextRange will return an error indicating that
		// there is no block to iterate
		rg, hasNext, err = progress.NextRange()
		require.NoError(t, err)
		require.False(t, hasNext)

		// now initialize again with a different latest that which cause latest < next
		// verify there will be no block to iterate
		initializer = store.NewConsumerProgress(pebbleimpl.ToDB(db), "test")
		progress, err = NewPersistentIteratorState(initializer, root, getLatest)
		require.NoError(t, err)
		_, hasNext, err = progress.NextRange()
		require.NoError(t, err)
		require.False(t, hasNext) // has no block to iterate
	})
}
