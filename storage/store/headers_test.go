package store_test

import (
	"testing"

	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestHeaderStoreRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		all := store.InitAll(metrics, db)
		headers := all.Headers
		blocks := all.Blocks

		block := unittest.BlockFixture()

		lctx := lockManager.NewContext()
		err := lctx.AcquireLock(storage.LockInsertBlock)
		require.NoError(t, err)
		defer lctx.Release()

		// store block which will also store header
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx, rw, &block)
		})
		require.NoError(t, err)

		lctx2 := lockManager.NewContext()
		require.NoError(t, lctx2.AcquireLock(storage.LockFinalizeBlock))
		// index the header
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexFinalizedBlockByHeight(lctx2, rw, block.Header.Height, block.ID())
		})
		lctx2.Release()
		require.NoError(t, err)

		// retrieve header by height
		actual, err := headers.ByHeight(block.Header.Height)
		require.NoError(t, err)
		require.Equal(t, block.Header, actual)
	})
}

func TestHeaderRetrieveWithoutStore(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		headers := store.NewHeaders(metrics, db)

		header := unittest.BlockHeaderFixture()

		// retrieve header by height, should err as not store before height
		_, err := headers.ByHeight(header.Height)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}
