package pebble

import (
	"os"
	"testing"

	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestIsDBNotBootstrapped(t *testing.T) {
	t.Parallel()
	dir := unittest.TempPebblePath(t)
	db, err := OpenRegisterPebbleDB(dir)
	require.NoError(t, err)
	notBootstrapped, err := IsDBNotBootstrapped(db)
	require.NoError(t, err)
	require.True(t, notBootstrapped)
	require.NoError(t, db.Close())
	err = os.RemoveAll(dir)
}

func TestReadHeightsFromBootstrappedDB(t *testing.T) {
	t.Parallel()
	dir := unittest.TempPebblePath(t)
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

	err = os.RemoveAll(dir)
}
