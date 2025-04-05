package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlockStoreAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		cacheMetrics := &metrics.NoopCollector{}
		// verify after storing a block should be able to retrieve it back
		blocks := badgerstorage.InitAll(cacheMetrics, db).Blocks
		block := unittest.FullBlockFixture()
		block.SetPayload(unittest.PayloadFixture(unittest.WithAllTheFixins))
		prop := unittest.ProposalFromBlock(&block)

		err := blocks.Store(prop)
		require.NoError(t, err)

		retrieved, err := blocks.ByID(block.ID())
		require.NoError(t, err)
		require.Equal(t, &block, retrieved)

		retrievedp, err := blocks.ProposalByID(block.ID())
		require.NoError(t, err)
		require.Equal(t, prop, retrievedp)

		// verify after a restart, the block stored in the database is the same
		// as the original
		blocksAfterRestart := badgerstorage.InitAll(cacheMetrics, db).Blocks
		receivedAfterRestart, err := blocksAfterRestart.ByID(block.ID())
		require.NoError(t, err)
		require.Equal(t, &block, receivedAfterRestart)

		receivedAfterRestartp, err := blocksAfterRestart.ProposalByID(block.ID())
		require.NoError(t, err)
		require.Equal(t, prop, receivedAfterRestartp)
	})
}
