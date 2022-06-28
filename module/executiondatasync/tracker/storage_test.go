package tracker

import (
	"crypto/rand"
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

func TestPrune(t *testing.T) {
	expectedPrunedCIDs := make(map[cid.Cid]struct{})
	storage, err := OpenStorage("/tmp/prune_test", 0, zerolog.Nop(), WithPruneCallback(func(c cid.Cid) error {
		_, ok := expectedPrunedCIDs[c]
		assert.True(t, ok, "unexpected CID pruned: %s", c.String())
		delete(expectedPrunedCIDs, c)
		return nil
	}))
	require.NoError(t, err)

	c1 := randomCid()
	expectedPrunedCIDs[c1] = struct{}{}
	c2 := randomCid()
	expectedPrunedCIDs[c2] = struct{}{}
	c3 := randomCid()
	c4 := randomCid()

	storage.Update(func(tbf TrackBlobsFn) error {
		require.NoError(t, tbf(1, c1, c2))
		require.NoError(t, tbf(2, c3, c4))

		return nil
	})
	require.NoError(t, storage.Prune(1))

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
