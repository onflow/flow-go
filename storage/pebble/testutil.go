package pebble

import (
	"sort"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func RunWithRegistersStorageAtInitialHeights(tb testing.TB, first uint64, latest uint64, f func(r *Registers)) {
	unittest.RunWithTempDir(tb, func(dir string) {
		db := NewBootstrappedRegistersWithPathForTest(tb, dir, first, latest)
		r, err := NewRegisters(db, PruningDisabled)
		require.NoError(tb, err)

		f(r)

		require.NoError(tb, db.Close())
	})
}

func NewBootstrappedRegistersWithPathForTest(tb testing.TB, dir string, first, latest uint64) *pebble.DB {
	db, err := OpenRegisterPebbleDB(dir)
	require.NoError(tb, err)

	// insert initial heights to pebble
	require.NoError(tb, db.Set(firstHeightKey, encodedUint64(first), nil))
	require.NoError(tb, db.Set(latestHeightKey, encodedUint64(latest), nil))
	return db
}

// RunWithRegistersStorageWithInitialData sets up a temporary register storage database
// initialized with the provided map of register entries and runs the provided test function `f`.
//
// This function creates a temporary database and initializes it with register entries at
// specific heights as defined in the `data` map. After initializing the database, it runs
// the test function `f` with the database instance and closes the database when done.
//
// Parameters:
//   - tb: The testing object used to manage test state and report errors.
//   - data: A map of heights to `RegisterEntries` that should be stored in the database at each height.
//   - f: A function to execute with the initialized `pebble.DB` instance.
//
// Example usage:
//
//		data := map[uint64]flow.RegisterEntries{
//		    100: flow.RegisterEntries {
//			    {Key: "key1", Value: []byte("value1")},
//			    {Key: "key2", Value: []byte("value2")},
//	 	    },
//		    200: flow.RegisterEntries {
//			    {Key: "key1", Value: []byte("value1")},
//			    {Key: "key3", Value: []byte("value3")},
//	 	    },
//		}
//		RunWithRegistersStorageWithInitialData(t, data, func(db *pebble.DB) {
//		    // Perform test operations with `db`
//		})
func RunWithRegistersStorageWithInitialData(
	tb testing.TB,
	data map[uint64]flow.RegisterEntries,
	f func(db *pebble.DB),
) {
	unittest.RunWithTempDir(tb, func(dir string) {
		db := NewBootstrappedRegistersWithPathForTest(tb, dir, uint64(0), uint64(0))
		registers, err := NewRegisters(db, 5)
		require.NoError(tb, err)

		heights := make([]uint64, 0, len(data))
		for h := range data {
			heights = append(heights, h)
		}
		// Should sort heights before store them through register, as they are not stored in the test data map
		sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })

		// Iterate over the heights in ascending order and store keys in DB through registers
		for _, height := range heights {
			err = registers.Store(data[height], height)
			require.NoError(tb, err)
		}

		f(db)

		require.NoError(tb, db.Close())
	})
}
