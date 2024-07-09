package badger

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestClusterBlocksByHeight(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		chain := unittest.ClusterBlockChainFixture(5)
		parent, blocks := chain[0], chain[1:]

		// add parent as boundary
		err := db.Update(operation.IndexClusterBlockHeight(parent.Header.ChainID, parent.Header.Height, parent.ID()))
		require.NoError(t, err)

		err = db.Update(operation.InsertClusterFinalizedHeight(parent.Header.ChainID, parent.Header.Height))
		require.NoError(t, err)

		// store a chain of blocks
		for _, block := range blocks {
			err := db.Update(procedure.InsertClusterBlock(&block))
			require.NoError(t, err)

			err = db.Update(procedure.FinalizeClusterBlock(block.Header.ID()))
			require.NoError(t, err)
		}

		clusterBlocks := NewClusterBlocks(
			db,
			blocks[0].Header.ChainID,
			NewHeaders(metrics.NewNoopCollector(), db),
			NewClusterPayloads(metrics.NewNoopCollector(), db),
		)

		// check if the block can be retrieved by height
		for _, block := range blocks {
			retrievedBlock, err := clusterBlocks.ByHeight(block.Header.Height)
			require.NoError(t, err)
			require.Equal(t, block.ID(), retrievedBlock.ID())
		}
	})
}
