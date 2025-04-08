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
		err := registers.Store(entries, 1)
		require.NoError(t, err)
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return registers.AddToBatch(rw)
		}))

		// Encode key
		encodedHeight := make([]byte, 8)
		binary.BigEndian.PutUint64(encodedHeight, height)
		key := append(encodedHeight, entries[0].Key.Bytes()...)

		// Get value
		reader := db.Reader()
		value, closer, err := reader.Get(key)
		defer closer.Close()
		require.NoError(t, err)

		// Ensure value with such a key was stored in DB
		require.NotEmpty(t, value)
	})
}
