package badger

import (
	"crypto/rand"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

func randomCid() cid.Cid {
	data := make([]byte, 1024)
	_, _ = rand.Read(data)
	return blobs.NewBlob(data).Cid()
}

// TestPrune tests that when a height is pruned, all CIDs appearing at or below the pruned
// height, and their associated tracking data, should be removed from the database.
func TestPrune(t *testing.T) {
	expectedPrunedCIDs := make(map[cid.Cid]struct{})
	storageDir := t.TempDir()
	executionDataTracker, err := NewExecutionDataTracker(storageDir, 0, zerolog.Nop(), WithPruneCallback(func(c cid.Cid) error {
		_, ok := expectedPrunedCIDs[c]
		assert.True(t, ok, "unexpected CID pruned: %s", c.String())
		delete(expectedPrunedCIDs, c)
		return nil
	}))
	require.NoError(t, err)

	// c1 and c2 are for height 1, and c3 and c4 are for height 2
	// after pruning up to height 1, only c1 and c2 should be pruned
	c1 := randomCid()
	expectedPrunedCIDs[c1] = struct{}{}
	c2 := randomCid()
	expectedPrunedCIDs[c2] = struct{}{}
	c3 := randomCid()
	c4 := randomCid()

	require.NoError(t, executionDataTracker.Update(func(tbf tracker.TrackBlobsFn) error {
		require.NoError(t, tbf(1, c1, c2))
		require.NoError(t, tbf(2, c3, c4))

		return nil
	}))
	require.NoError(t, executionDataTracker.PruneUpToHeight(1))

	prunedHeight, err := executionDataTracker.GetPrunedHeight()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), prunedHeight)

	assert.Len(t, expectedPrunedCIDs, 0)

	var latestHeight uint64
	err = executionDataTracker.db.View(func(txn *badger.Txn) error {
		err = operation.RetrieveBlob(1, c1)(txn)
		assert.ErrorIs(t, err, storage.ErrNotFound)
		err = operation.RetrieveTrackerLatestHeight(c1, &latestHeight)(txn)
		assert.ErrorIs(t, err, storage.ErrNotFound)
		err = operation.RetrieveBlob(1, c2)(txn)
		assert.ErrorIs(t, err, storage.ErrNotFound)
		err = operation.RetrieveTrackerLatestHeight(c2, &latestHeight)(txn)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		err = operation.RetrieveBlob(2, c3)(txn)
		assert.NoError(t, err)
		err = operation.RetrieveTrackerLatestHeight(c3, &latestHeight)(txn)
		assert.NoError(t, err)
		err = operation.RetrieveBlob(2, c4)(txn)
		assert.NoError(t, err)
		err = operation.RetrieveTrackerLatestHeight(c4, &latestHeight)(txn)
		assert.NoError(t, err)

		return nil
	})
	require.NoError(t, err)
}

// TestPruneNonLatestHeight test that when pruning a height at which a CID exists,
// if that CID also exists at another height above the pruned height, the CID should not be pruned.
func TestPruneNonLatestHeight(t *testing.T) {
	storageDir := t.TempDir()
	storage, err := NewExecutionDataTracker(storageDir, 0, zerolog.Nop(), WithPruneCallback(func(c cid.Cid) error {
		assert.Fail(t, "unexpected CID pruned: %s", c.String())
		return nil
	}))
	require.NoError(t, err)

	// c1 and c2 appear both at height 1 and 2
	// therefore, when pruning up to height 1, both c1 and c2 should be retained
	c1 := randomCid()
	c2 := randomCid()

	require.NoError(t, storage.Update(func(tbf tracker.TrackBlobsFn) error {
		require.NoError(t, tbf(1, c1, c2))
		require.NoError(t, tbf(2, c1, c2))

		return nil
	}))
	require.NoError(t, storage.PruneUpToHeight(1))

	prunedHeight, err := storage.GetPrunedHeight()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), prunedHeight)

	var latestHeight uint64
	err = storage.db.View(func(txn *badger.Txn) error {
		err = operation.RetrieveBlob(2, c1)(txn)
		assert.NoError(t, err)
		err = operation.RetrieveTrackerLatestHeight(c1, &latestHeight)(txn)
		assert.NoError(t, err)
		err = operation.RetrieveBlob(2, c2)(txn)
		assert.NoError(t, err)
		err = operation.RetrieveTrackerLatestHeight(c2, &latestHeight)(txn)
		assert.NoError(t, err)

		return nil
	})
	require.NoError(t, err)
}

// TestAscendingOrderOfRecords tests that order of data is ascending and all CIDs appearing at or below the pruned
// height, and their associated tracking data, should be removed from the database.
func TestAscendingOrderOfRecords(t *testing.T) {
	expectedPrunedCIDs := make(map[cid.Cid]struct{})
	storageDir := t.TempDir()
	executionDataTracker, err := NewExecutionDataTracker(storageDir, 0, zerolog.Nop(), WithPruneCallback(func(c cid.Cid) error {
		_, ok := expectedPrunedCIDs[c]
		assert.True(t, ok, "unexpected CID pruned: %s", c.String())
		delete(expectedPrunedCIDs, c)
		return nil
	}))
	require.NoError(t, err)

	// c1 is for height 1,
	// c2 is for height 2,
	// c3 is for height 256
	// pruning up to height 1 will check if order of the records is ascending, c1 should be pruned
	c1 := randomCid()
	expectedPrunedCIDs[c1] = struct{}{}
	c2 := randomCid()
	c3 := randomCid()

	require.NoError(t, executionDataTracker.Update(func(tbf tracker.TrackBlobsFn) error {
		require.NoError(t, tbf(1, c1))
		require.NoError(t, tbf(2, c2))
		// It is important to check if the record with height 256 does not precede
		// the record with height 1 during pruning.
		require.NoError(t, tbf(256, c3))

		return nil
	}))
	require.NoError(t, executionDataTracker.PruneUpToHeight(1))

	prunedHeight, err := executionDataTracker.GetPrunedHeight()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), prunedHeight)

	assert.Len(t, expectedPrunedCIDs, 0)

	var latestHeight uint64
	err = executionDataTracker.db.View(func(txn *badger.Txn) error {
		// expected that blob record with height 1 was removed
		err = operation.RetrieveBlob(1, c1)(txn)
		assert.ErrorIs(t, err, storage.ErrNotFound)
		err = operation.RetrieveTrackerLatestHeight(c1, &latestHeight)(txn)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		// expected that blob record with height 2 exists
		err = operation.RetrieveBlob(2, c2)(txn)
		assert.NoError(t, err)
		err = operation.RetrieveTrackerLatestHeight(c2, &latestHeight)(txn)
		assert.NoError(t, err)

		// expected that blob record with height 256 exists
		err = operation.RetrieveBlob(256, c3)(txn)
		assert.NoError(t, err)
		err = operation.RetrieveTrackerLatestHeight(c3, &latestHeight)(txn)
		assert.NoError(t, err)

		return nil
	})
	require.NoError(t, err)
}
