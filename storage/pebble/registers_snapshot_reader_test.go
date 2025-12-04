package pebble

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// TestRegisterSnapshotReader_StorageSnapshot verifies the behavior of the
// RegisterSnapshotReader when constructing register snapshots at a given
// block height.
//
// Test cases:
//  1. Heights below the first indexed height, return storage.ErrHeightNotIndexed.
//  2. Heights above the latest indexed height, return storage.ErrHeightNotIndexed.
//  3. Snapshots at valid heights, return stored values correctly.
//  4. Snapshots at valid heights, return nil (not an error) when a
//     register key is missing.
func TestRegisterSnapshotReader_StorageSnapshot(t *testing.T) {
	RunWithRegistersStorageAtHeight1(t, func(r *Registers) {
		reader := NewRegisterSnapshotReader(r)

		// 1: Requesting a snapshot below the first indexed height should
		// fail with storage.ErrHeightNotIndexed.
		t.Run("error when height below first indexed", func(t *testing.T) {
			_, err := reader.StorageSnapshot(r.FirstHeight() - 1)
			require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
		})

		// 2: Requesting a snapshot above the latest indexed height should
		// fail with storage.ErrHeightNotIndexed.
		t.Run("error when height above latest indexed", func(t *testing.T) {
			_, err := reader.StorageSnapshot(r.LatestHeight() + 1)
			require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
		})

		// 3: When a snapshot is requested at a valid height, the snapshot
		// should return the stored register value without error.
		t.Run("returns value at valid height", func(t *testing.T) {
			key := flow.RegisterID{Owner: "owner", Key: "key"}
			value := []byte("value")
			err := r.Store(flow.RegisterEntries{{Key: key, Value: value}}, r.LatestHeight()+1)
			require.NoError(t, err)

			height := r.LatestHeight()
			snap, err := reader.StorageSnapshot(height)
			require.NoError(t, err)

			got, err := snap.Get(key)
			require.NoError(t, err)
			require.Equal(t, value, got)
		})

		// 4: When a snapshot is requested for a valid height but the key
		// does not exist, the snapshot should return nil without an error.
		t.Run("returns nil for missing key", func(t *testing.T) {
			height := r.LatestHeight()
			snap, err := reader.StorageSnapshot(height)
			require.NoError(t, err)

			missingKey := flow.RegisterID{Owner: "owner", Key: "missing"}
			got, err := snap.Get(missingKey)
			require.NoError(t, err)
			require.Nil(t, got)
		})
	})
}
