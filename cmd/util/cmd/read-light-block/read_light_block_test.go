package read

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/storage/pebble/procedure"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReadClusterRange(t *testing.T) {

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		chain := unittest.ClusterBlockChainFixture(5)
		parent, blocks := chain[0], chain[1:]

		// add parent as boundary
		err := operation.IndexClusterBlockHeight(parent.Header.ChainID, parent.Header.Height, parent.ID())(db)
		require.NoError(t, err)

		err = operation.InsertClusterFinalizedHeight(parent.Header.ChainID, parent.Header.Height)(db)
		require.NoError(t, err)

		// add blocks
		for _, block := range blocks {
			err := operation.WithReaderBatchWriter(db, procedure.InsertClusterBlock(&block))
			require.NoError(t, err)

			err = operation.WithReaderBatchWriter(db, procedure.FinalizeClusterBlock(block.Header.ID()))
			require.NoError(t, err)
		}

		clusterBlocks := pebblestorage.NewClusterBlocks(
			db,
			blocks[0].Header.ChainID,
			pebblestorage.NewHeaders(metrics.NewNoopCollector(), db),
			pebblestorage.NewClusterPayloads(metrics.NewNoopCollector(), db),
		)

		startHeight := blocks[0].Header.Height
		endHeight := startHeight + 10 // if end height is exceeded the last finalized height, only return up to the last finalized
		lights, err := ReadClusterLightBlockByHeightRange(clusterBlocks, startHeight, endHeight)
		require.NoError(t, err)

		for i, light := range lights {
			require.Equal(t, light.ID, blocks[i].ID())
		}

		require.Equal(t, len(blocks), len(lights))
	})
}
