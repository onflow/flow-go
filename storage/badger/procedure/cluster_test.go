package procedure

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInsertRetrieveClusterBlock(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		block := unittest.ClusterBlockFixture()

		err := db.Update(InsertClusterBlock(unittest.ClusterProposalFromBlock(block)))
		require.NoError(t, err)

		var retrieved cluster.Block
		err = db.View(RetrieveClusterBlock(block.ID(), &retrieved))
		require.NoError(t, err)

		require.Equal(t, *block, retrieved)
	})
}

func TestFinalizeClusterBlock(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		parent := unittest.ClusterBlockFixture()

		block := unittest.ClusterBlockFixture(
			unittest.ClusterBlock.WithParent(parent),
		)

		err := db.Update(InsertClusterBlock(unittest.ClusterProposalFromBlock(block)))
		require.NoError(t, err)

		err = db.Update(operation.IndexClusterBlockHeight(block.ChainID, parent.Height, parent.ID()))
		require.NoError(t, err)

		err = db.Update(operation.InsertClusterFinalizedHeight(block.ChainID, parent.Height))
		require.NoError(t, err)

		err = db.Update(FinalizeClusterBlock(block.ID()))
		require.NoError(t, err)

		var boundary uint64
		err = db.View(operation.RetrieveClusterFinalizedHeight(block.ChainID, &boundary))
		require.NoError(t, err)
		require.Equal(t, block.Height, boundary)

		var headID flow.Identifier
		err = db.View(operation.LookupClusterBlockHeight(block.ChainID, boundary, &headID))
		require.NoError(t, err)
		require.Equal(t, block.ID(), headID)
	})
}
