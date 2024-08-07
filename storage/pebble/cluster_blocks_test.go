package pebble

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/storage/pebble/procedure"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestClusterBlocksByHeight(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		chain := unittest.ClusterBlockChainFixture(5)
		parent, blocks := chain[0], chain[1:]

		// add parent as boundary
		err := operation.IndexClusterBlockHeight(parent.Header.ChainID, parent.Header.Height, parent.ID())(db)
		require.NoError(t, err)

		err = operation.InsertClusterFinalizedHeight(parent.Header.ChainID, parent.Header.Height)(db)
		require.NoError(t, err)

		// store a chain of blocks
		for _, block := range blocks {
			err := operation.WithReaderBatchWriter(db, procedure.InsertClusterBlock(&block))
			require.NoError(t, err)

			err = operation.WithReaderBatchWriter(db, procedure.FinalizeClusterBlock(block.Header.ID()))
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
