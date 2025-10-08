package inmemory

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRegisters_HappyPath(t *testing.T) {
	// Prepare register entries
	entry1 := unittest.RegisterEntryFixture()
	entry1.Key = flow.RegisterID{Owner: "owner1", Key: "key1"}

	entry2 := unittest.RegisterEntryFixture()
	entry2.Key = flow.RegisterID{Owner: "owner2", Key: "key2"}

	entries := flow.RegisterEntries{entry1, entry2}

	height := uint64(42)
	registers := NewRegisters(height, entries)

	require.Equal(t, height, registers.FirstHeight())
	require.Equal(t, height, registers.LatestHeight())

	// Retrieve both entries
	got1, err := registers.Get(entry1.Key, height)
	require.NoError(t, err)
	require.Equal(t, entry1.Value, got1)

	got2, err := registers.Get(entry2.Key, height)
	require.NoError(t, err)
	require.Equal(t, entry2.Value, got2)

	// Try retrieving at the wrong height
	_, err = registers.Get(entry1.Key, height+1)
	require.ErrorIs(t, err, storage.ErrHeightNotIndexed)

	// Try getting a non-existent key
	_, err = registers.Get(unittest.RegisterIDFixture(), height)
	require.ErrorIs(t, err, storage.ErrNotFound)
}
