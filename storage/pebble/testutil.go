package pebble

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

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
