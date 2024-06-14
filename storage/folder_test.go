package storage_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCheckFolderIsEmpty(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		isPebble, isBadger, isEmpty, err := storage.CheckFolder(dir)
		require.NoError(t, err)

		require.True(t, isEmpty)
		require.False(t, isBadger)
		require.False(t, isPebble)
	})
}

func TestCheckFolderIsBadger(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		db := unittest.BadgerDB(t, dir)
		require.NoError(t, db.Close())

		isPebble, isBadger, isEmpty, err := storage.CheckFolder(dir)
		require.NoError(t, err)

		require.False(t, isEmpty)
		require.True(t, isBadger)
		require.False(t, isPebble)
	})
}

func TestCheckFolderIsPebble(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		fmt.Println("dir", dir)
		db, err := pebble.OpenDefaultPebbleDB(dir)
		require.NoError(t, err)
		require.NoError(t, db.Close())

		isPebble, isBadger, isEmpty, err := storage.CheckFolder(dir)
		require.NoError(t, err)

		require.False(t, isEmpty)
		require.False(t, isBadger)
		require.True(t, isPebble)
	})
}
