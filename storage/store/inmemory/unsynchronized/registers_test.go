package unsynchronized

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRegisters_HappyPath(t *testing.T) {
	height := uint64(42)
	registers := NewRegisters(height)

	require.Equal(t, height, registers.FirstHeight())
	require.Equal(t, height, registers.LatestHeight())

	// Prepare register entries
	entry1 := unittest.RegisterEntryFixture()
	entry1.Key = flow.RegisterID{Owner: "owner1", Key: "key1"}

	entry2 := unittest.RegisterEntryFixture()
	entry2.Key = flow.RegisterID{Owner: "owner2", Key: "key2"}

	entries := flow.RegisterEntries{entry1, entry2}

	// Store entries (height is ignored)
	err := registers.Store(entries, height)
	require.NoError(t, err)

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

	// Try storing at the wrong height
	err = registers.Store(entries, height+1)
	require.ErrorIs(t, err, storage.ErrHeightNotIndexed)

	// Try getting a non-existent key
	_, err = registers.Get(unittest.RegisterIDFixture(), height)
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func TestRegisters_Persist(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		height := uint64(1)
		registers := NewRegisters(height)

		entries := flow.RegisterEntries{unittest.RegisterEntryFixture()}
		entries[0].Key = flow.RegisterID{
			Owner: "owner1",
			Key:   "key1",
		}

		// Persist registers
		err := registers.Store(entries, height)
		require.NoError(t, err)
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return registers.AddToBatch(rw)
		}))

		// Encode key
		encodedHeight := make([]byte, 8)
		binary.BigEndian.PutUint64(encodedHeight, height)
		key := append(encodedHeight, entries[0].Key.Bytes()...)

		// Get value
		reader, err := db.Reader()
		require.NoError(t, err)

		value, closer, err := reader.Get(key)
		defer closer.Close()
		require.NoError(t, err)

		// Ensure value with such a key was stored in DB
		require.NotEmpty(t, value)
	})
}
