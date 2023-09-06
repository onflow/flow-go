package read

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReadClusterRange(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		chain := unittest.ClusterBlockChainFixture(5)
		parent, blocks := chain[0], chain[1:]

		// add parent as boundary
		err := db.Update(operation.IndexClusterBlockHeight(parent.Header.ChainID, parent.Header.Height, parent.ID()))
		require.NoError(t, err)

		err = db.Update(operation.InsertClusterFinalizedHeight(parent.Header.ChainID, parent.Header.Height))
		require.NoError(t, err)

		// add blocks
		for _, block := range blocks {
			err := db.Update(procedure.InsertClusterBlock(&block))
			require.NoError(t, err)

			err = db.Update(procedure.FinalizeClusterBlock(block.Header.ID()))
			require.NoError(t, err)
		}

		clusterBlocks := badgerstorage.NewClusterBlocks(
			db,
			blocks[0].Header.ChainID,
			badgerstorage.NewHeaders(metrics.NewNoopCollector(), db),
			badgerstorage.NewClusterPayloads(metrics.NewNoopCollector(), db),
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
