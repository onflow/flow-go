package pebble

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIsBootstrapped(t *testing.T) {
	t.Parallel()
	unittest.RunWithTempDir(t, func(dir string) {
		logger := unittest.Logger()
		db, err := OpenRegisterPebbleDB(logger, dir)
		require.NoError(t, err)
		bootstrapped, err := IsBootstrapped(db)
		require.NoError(t, err)
		require.False(t, bootstrapped)
		require.NoError(t, db.Close())
	})
}

func TestReadHeightsFromBootstrappedDB(t *testing.T) {
	t.Parallel()
	unittest.RunWithTempDir(t, func(dir string) {
		logger := unittest.Logger()
		db, err := OpenRegisterPebbleDB(logger, dir)
		require.NoError(t, err)

		// init with first height
		firstHeight := uint64(10)
		require.NoError(t, initHeights(db, firstHeight))

		bootstrapped, err := IsBootstrapped(db)
		require.NoError(t, err)

		require.True(t, bootstrapped)
		require.NoError(t, db.Close())

		// reopen the db
		registers, db, err := NewBootstrappedRegistersWithPath(logger, dir)
		require.NoError(t, err)

		require.Equal(t, firstHeight, registers.FirstHeight())
		require.Equal(t, firstHeight, registers.LatestHeight())

		require.NoError(t, db.Close())
	})
}

func TestNewBootstrappedRegistersWithPath(t *testing.T) {
	t.Parallel()
	unittest.RunWithTempDir(t, func(dir string) {
		logger := unittest.Logger()
		_, db, err := NewBootstrappedRegistersWithPath(logger, dir)
		require.ErrorIs(t, err, storage.ErrNotBootstrapped)

		// verify the db is closed
		require.True(t, db == nil)

		// bootstrap the db
		// init with first height
		db2, err := OpenRegisterPebbleDB(logger, dir)
		require.NoError(t, err)
		firstHeight := uint64(10)
		require.NoError(t, initHeights(db2, firstHeight))

		registers, err := NewRegisters(db2, PruningDisabled)
		require.NoError(t, err)
		require.Equal(t, firstHeight, registers.FirstHeight())
		require.Equal(t, firstHeight, registers.LatestHeight())

		require.NoError(t, db2.Close())
	})
}

func TestMustOpenDefaultPebbleDB(t *testing.T) {
	t.Parallel()
	unittest.RunWithTempDir(t, func(dir string) {
		logger := unittest.Logger()
		// verify error is returned when the db is not bootstrapped
		_, err := MustOpenDefaultPebbleDB(logger, dir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not initialized")

		// bootstrap the db
		db, err := OpenDefaultPebbleDB(logger, dir)
		require.NoError(t, err)
		require.NoError(t, initHeights(db, uint64(10)))
		require.NoError(t, db.Close())
		fmt.Println(dir)

		// verify no error is returned when the db is bootstrapped
		db, err = MustOpenDefaultPebbleDB(logger, dir)
		require.NoError(t, err)

		h, err := latestStoredHeight(db)
		require.NoError(t, err)
		require.Equal(t, uint64(10), h)
		require.NoError(t, db.Close())
	})
}
