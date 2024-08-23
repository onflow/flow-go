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
		r, err := NewRegisters(db)
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

func RunWithRegistersStorageWithInitialData(
	tb testing.TB,
	data map[uint64]flow.RegisterEntries,
	f func(db *pebble.DB)) {
	unittest.RunWithTempDir(tb, func(dir string) {
		db := NewBootstrappedRegistersWithPathForTest(tb, dir, uint64(0), uint64(0))
		r, err := NewRegisters(db)
		require.NoError(tb, err)

		keys := make([]uint64, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

		// Iterate over the map in ascending order by key
		for _, height := range keys {
			err = r.Store(data[height], height)
			require.NoError(tb, err)
		}

		f(db)

		require.NoError(tb, db.Close())
	})
}
