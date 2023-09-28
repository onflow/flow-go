package pebble

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestIsDBNotBootstrapped(t *testing.T) {
	t.Parallel()
	unittest.RunWithTempDir(t, func(dir string) {
		db, err := OpenRegisterPebbleDB(dir)
		require.NoError(t, err)
		notBootstrapped, err := IsDBNotBootstrapped(db)
		require.NoError(t, err)
		require.True(t, notBootstrapped)
		require.NoError(t, db.Close())
	})
}

func TestReadHeightsFromBootstrappedDB(t *testing.T) {
	t.Parallel()
	unittest.RunWithTempDir(t, func(dir string) {
		db, err := OpenRegisterPebbleDB(dir)
		require.NoError(t, err)

		// init with first height
		firstHeight := uint64(10)
		require.NoError(t, initHeights(db, firstHeight))

		notBootstrapped, err := IsDBNotBootstrapped(db)
		require.NoError(t, err)

		require.False(t, notBootstrapped)
		require.NoError(t, db.Close())

		// reopen the db
		registers, db, err := NewBootstrappedRegistersWithPath(dir)
		require.NoError(t, err)

		require.Equal(t, firstHeight, registers.FirstHeight())
		require.Equal(t, firstHeight, registers.LatestHeight())

		require.NoError(t, db.Close())
	})
}

func TestNewBootstrappedRegistersWithPath(t *testing.T) {
	t.Parallel()
	unittest.RunWithTempDir(t, func(dir string) {
		_, db, err := NewBootstrappedRegistersWithPath(dir)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrNotBootstrapped))

		// verify the db is closed
		require.True(t, db == nil)

		// bootstrap the db
		// init with first height
		db2, err := OpenRegisterPebbleDB(dir)
		require.NoError(t, err)
		firstHeight := uint64(10)
		require.NoError(t, initHeights(db2, firstHeight))

		registers, err := NewRegisters(db2)
		require.NoError(t, err)
		require.Equal(t, firstHeight, registers.FirstHeight())
		require.Equal(t, firstHeight, registers.LatestHeight())

		require.NoError(t, db2.Close())
	})
}
