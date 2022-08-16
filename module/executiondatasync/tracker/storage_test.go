package tracker

import (
	"crypto/rand"
	"io/ioutil"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/blobs"
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
	storageDir, err := ioutil.TempDir("/tmp", "prune_test")
	require.NoError(t, err)
	storage, err := OpenStorage(storageDir, 0, zerolog.Nop(), WithPruneCallback(func(c cid.Cid) error {
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

	require.NoError(t, storage.Update(func(tbf TrackBlobsFn) error {
		require.NoError(t, tbf(1, c1, c2))
		require.NoError(t, tbf(2, c3, c4))

		return nil
	}))
	require.NoError(t, storage.PruneUpToHeight(1))

	prunedHeight, err := storage.GetPrunedHeight()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), prunedHeight)

	assert.Len(t, expectedPrunedCIDs, 0)

	err = storage.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(makeBlobRecordKey(1, c1))
		assert.ErrorIs(t, err, badger.ErrKeyNotFound)
		_, err = txn.Get(makeLatestHeightKey(c1))
		assert.ErrorIs(t, err, badger.ErrKeyNotFound)
		_, err = txn.Get(makeBlobRecordKey(1, c2))
		assert.ErrorIs(t, err, badger.ErrKeyNotFound)
		_, err = txn.Get(makeLatestHeightKey(c2))
		assert.ErrorIs(t, err, badger.ErrKeyNotFound)

		_, err = txn.Get(makeBlobRecordKey(2, c3))
		assert.NoError(t, err)
		_, err = txn.Get(makeLatestHeightKey(c3))
		assert.NoError(t, err)
		_, err = txn.Get(makeBlobRecordKey(2, c4))
		assert.NoError(t, err)
		_, err = txn.Get(makeLatestHeightKey(c4))
		assert.NoError(t, err)

		return nil
	})
	require.NoError(t, err)
}

// TestPruneNonLatestHeight test that when pruning a height at which a CID exists,
// if that CID also exists at another height above the pruned height, the CID should not be pruned.
func TestPruneNonLatestHeight(t *testing.T) {
	storageDir, err := ioutil.TempDir("/tmp", "prune_test")
	require.NoError(t, err)
	storage, err := OpenStorage(storageDir, 0, zerolog.Nop(), WithPruneCallback(func(c cid.Cid) error {
		assert.Fail(t, "unexpected CID pruned: %s", c.String())
		return nil
	}))
	require.NoError(t, err)

	// c1 and c2 appear both at height 1 and 2
	// therefore, when pruning up to height 1, both c1 and c2 should be retained
	c1 := randomCid()
	c2 := randomCid()

	require.NoError(t, storage.Update(func(tbf TrackBlobsFn) error {
		require.NoError(t, tbf(1, c1, c2))
		require.NoError(t, tbf(2, c1, c2))

		return nil
	}))
	require.NoError(t, storage.PruneUpToHeight(1))

	prunedHeight, err := storage.GetPrunedHeight()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), prunedHeight)

	err = storage.db.View(func(txn *badger.Txn) error {
		_, err = txn.Get(makeBlobRecordKey(2, c1))
		assert.NoError(t, err)
		_, err = txn.Get(makeLatestHeightKey(c1))
		assert.NoError(t, err)
		_, err = txn.Get(makeBlobRecordKey(2, c2))
		assert.NoError(t, err)
		_, err = txn.Get(makeLatestHeightKey(c2))
		assert.NoError(t, err)

		return nil
	})
	require.NoError(t, err)
}
