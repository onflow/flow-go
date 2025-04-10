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
	registers := NewRegisters(firstHeight)

	// Ensure initial heights are correct
	require.Equal(t, firstHeight, registers.FirstHeight())
	require.Equal(t, uint64(0), registers.LatestHeight())

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

	// Store entries at height 1
	err := registers.Store(entries, 1)
	require.NoError(t, err)

	// Verify latest height is updated
	require.Equal(t, uint64(1), registers.LatestHeight())

	// Retrieve stored value
	val, err := registers.Get(entries[0].Key, 1)
	require.NoError(t, err)
	require.Equal(t, entries[0].Value, val)

	// Ensure retrieving a non-existent height returns an error
	_, err = registers.Get(unittest.RegisterIDFixture(), 2)
	require.ErrorIs(t, err, storage.ErrNotFound)

	// Ensure retrieving a non-existent register ID returns an error
	_, err = registers.Get(unittest.RegisterIDFixture(), 1)
	require.ErrorIs(t, err, storage.ErrNotFound)
}
