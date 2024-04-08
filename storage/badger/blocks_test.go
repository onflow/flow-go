package badger_test

import (
	"context"
	"errors"
	"path"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlocks(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := badgerstorage.NewBlocks(db, nil, nil)

		// check retrieval of non-existing key
		_, err := store.GetLastFullBlockHeight()
		assert.Error(t, err)
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// insert a value for height
		var height1 = uint64(1234)
		err = store.UpdateLastFullBlockHeight(height1)
		assert.NoError(t, err)

		// check value can be retrieved
		actual, err := store.GetLastFullBlockHeight()
		assert.NoError(t, err)
		assert.Equal(t, height1, actual)

		// update the value for height
		var height2 = uint64(1234)
		err = store.UpdateLastFullBlockHeight(height2)
		assert.NoError(t, err)

		// check that the new value can be retrieved
		actual, err = store.GetLastFullBlockHeight()
		assert.NoError(t, err)
		assert.Equal(t, height2, actual)
	})
}

func TestBlockStoreAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		cacheMetrics := &metrics.NoopCollector{}
		// verify after storing a block should be able to retrieve it back
		blocks := badgerstorage.InitAll(cacheMetrics, db).Blocks
		block := unittest.FullBlockFixture()
		block.SetPayload(unittest.PayloadFixture(unittest.WithAllTheFixins))

		err := blocks.Store(&block)
		require.NoError(t, err)

		retrieved, err := blocks.ByID(block.ID())
		require.NoError(t, err)

		require.Equal(t, &block, retrieved)

		// verify after a restart, the block stored in the database is the same
		// as the original
		blocksAfterRestart := badgerstorage.InitAll(cacheMetrics, db).Blocks
		receivedAfterRestart, err := blocksAfterRestart.ByID(block.ID())
		require.NoError(t, err)

		require.Equal(t, &block, receivedAfterRestart)
	})
}

func TestBlockStoreAndRetrieveCompress(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		folder1 := path.Join(dir, "fold1")
		db1 := unittest.BadgerDB(t, folder1)
		defer func() {
			assert.NoError(t, db1.Close())
		}()

		folder2 := path.Join(dir, "fold2")
		db2 := unittest.BadgerDB(t, folder2)
		defer func() {
			assert.NoError(t, db2.Close())
		}()

		cacheMetrics := &metrics.NoopCollector{}
		block := unittest.FullBlockFixture()
		block.SetPayload(unittest.PayloadFixture(unittest.WithAllTheFixins))
		// verify after storing a block should be able to retrieve it back
		blocks := badgerstorage.InitAll(cacheMetrics, db1).Blocks
		err := blocks.Store(&block)
		require.NoError(t, err)

		keyvals := make(chan *operation.KeyValue, 10)
		batchMaxLen := 1000
		batchMaxByteSize := 10000
		g, gCtx := errgroup.WithContext(context.Background())
		g.Go(func() error {
			return operation.CompressAndStore(unittest.Logger(), gCtx, keyvals, db2, batchMaxLen, batchMaxByteSize)
		})

		err = operation.TraverseKeyValues(keyvals, db1)
		require.NoError(t, err)

		require.NoError(t, g.Wait())

		// operation.SetCompressEnabled()

		blocksAfterRestart := badgerstorage.InitAll(cacheMetrics, db2).Blocks
		receivedAfterRestart, err := blocksAfterRestart.ByID(block.ID())
		require.NoError(t, err)

		require.Equal(t, &block, receivedAfterRestart)

	})

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		cacheMetrics := &metrics.NoopCollector{}
		// verify after storing a block should be able to retrieve it back
		blocks := badgerstorage.InitAll(cacheMetrics, db).Blocks
		block := unittest.FullBlockFixture()
		block.SetPayload(unittest.PayloadFixture(unittest.WithAllTheFixins))

		err := blocks.Store(&block)
		require.NoError(t, err)

		// verify after a restart, the block stored in the database is the same
		// as the original
		blocksAfterRestart := badgerstorage.InitAll(cacheMetrics, db).Blocks
		receivedAfterRestart, err := blocksAfterRestart.ByID(block.ID())
		require.NoError(t, err)

		require.Equal(t, &block, receivedAfterRestart)
	})
}
