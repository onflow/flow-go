package block_iterator

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	storagepebble "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProgress(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		root := uint64(10)

		latest := uint64(20)
		getLatest := func() (uint64, error) {
			return latest, nil
		}

		store := storagepebble.NewConsumerProgress(db, "test")

		progress, err := NewPersistentIteratorState(store, root, getLatest)
		require.NoError(t, err)

		// initial state should be the next of root
		next, err := progress.LoadState()
		require.NoError(t, err)
		require.Equal(t, root+1, next)

		rg, err := progress.NextRange()
		require.NoError(t, err)
		require.Equal(t, root+1, rg.Start)
		require.Equal(t, latest, rg.End)

		// save the state
		err = progress.SaveState(latest + 1)
		require.NoError(t, err)

		next, err = progress.LoadState()
		require.NoError(t, err)
		require.Equal(t, latest+1, next)

		// update latest
		oldLatest := latest
		latest = latest + 20
		rg, err = progress.NextRange()
		require.NoError(t, err)

		// verify the new range
		require.Equal(t, oldLatest+1, rg.Start)
		require.Equal(t, latest, rg.End)
	})
}
