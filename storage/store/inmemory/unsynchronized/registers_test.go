package unsynchronized

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRegisters_HappyPath(t *testing.T) {
	firstHeight := uint64(1)
	latestHeight := firstHeight
	registers := NewRegisters(firstHeight, latestHeight, 100, 0.1)

	// Ensure initial heights are correct
	require.Equal(t, firstHeight, registers.FirstHeight())
	require.Equal(t, latestHeight, registers.LatestHeight())

	// Define register entries
	entries := flow.RegisterEntries{unittest.RegisterEntryFixture(), unittest.RegisterEntryFixture()}
	entries[0].Key = flow.RegisterID{
		Owner: "owner1",
		Key:   "key1",
	}
	entries[1].Key = flow.RegisterID{
		Owner: "owner2",
		Key:   "key2",
	}

	// Store entries at the new height
	newHeight := latestHeight + 1
	err := registers.Store(entries, newHeight)
	require.NoError(t, err)

	// Verify latest height is updated
	require.Equal(t, newHeight, registers.LatestHeight())

	// Retrieve stored value
	val, err := registers.Get(entries[0].Key, newHeight)
	require.NoError(t, err)
	require.Equal(t, entries[0].Value, val)

	// Ensure retrieving a non-existent height returns an error
	_, err = registers.Get(unittest.RegisterIDFixture(), newHeight+1)
	require.ErrorIs(t, err, storage.ErrHeightNotIndexed)

	// Ensure retrieving a non-existent register ID returns an error
	_, err = registers.Get(unittest.RegisterIDFixture(), newHeight)
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func TestRegisters_Get(t *testing.T) {
	firstHeight := uint64(5)
	latestHeight := firstHeight
	registers := NewRegisters(firstHeight, latestHeight, 100, 0.1)

	// Store at height 6
	entries := flow.RegisterEntries{unittest.RegisterEntryFixture()}
	entries[0].Key = flow.RegisterID{
		Owner: "owner1",
		Key:   "key1",
	}
	err := registers.Store(entries, 6)
	require.NoError(t, err)

	// Exists at exact height
	got, err := registers.Get(entries[0].Key, 6)
	require.NoError(t, err)
	require.Equal(t, entries[0].Value, got)

	// Exists at fallback height
	got, err = registers.Get(entries[0].Key, 7) // height not indexed
	require.ErrorIs(t, err, storage.ErrHeightNotIndexed)

	err = registers.Store(flow.RegisterEntries{}, 7) // index height 7 with no data
	require.NoError(t, err)

	got, err = registers.Get(entries[0].Key, 7)
	require.NoError(t, err)
	require.Equal(t, entries[0].Value, got)

	// Not found at any height
	nonExistent := unittest.RegisterEntryFixture()
	_, err = registers.Get(nonExistent.Key, 7)
	require.ErrorIs(t, err, storage.ErrNotFound)

	// Below first height
	_, err = registers.Get(nonExistent.Key, firstHeight-1)
	require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
}
