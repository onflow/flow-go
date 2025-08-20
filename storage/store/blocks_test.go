package store_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlockStoreAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		cacheMetrics := &metrics.NoopCollector{}
		// verify after storing a block should be able to retrieve it back
		blocks := store.InitAll(cacheMetrics, db).Blocks
		block := unittest.FullBlockFixture()
		prop := unittest.ProposalFromBlock(block)

		_, lctx := unittest.LockManagerWithContext(t, storage.LockInsertBlock)
		defer lctx.Release()
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx, rw, prop)
		})
		require.NoError(t, err)

		retrieved, err := blocks.ByID(block.ID())
		require.NoError(t, err)
		require.Equal(t, block, retrieved)

		retrievedp, err := blocks.ProposalByID(block.ID())
		require.NoError(t, err)
		require.Equal(t, prop, retrievedp)

		// verify after a restart, the block stored in the database is the same
		// as the original
		blocksAfterRestart := store.InitAll(cacheMetrics, db).Blocks
		receivedAfterRestart, err := blocksAfterRestart.ByID(block.ID())
		require.NoError(t, err)
		require.Equal(t, block, receivedAfterRestart)

		receivedAfterRestartp, err := blocksAfterRestart.ProposalByID(block.ID())
		require.NoError(t, err)
		require.Equal(t, prop, receivedAfterRestartp)
	})
}
