package procedure

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestInsertRetrieveClusterBlock(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		block := unittest.ClusterBlockFixture()

		err := db.Update(InsertClusterBlock(&block))
		require.NoError(t, err)

		var retrieved cluster.Block
		err = db.View(RetrieveClusterBlock(block.Header.ID(), &retrieved))
		require.NoError(t, err)

		require.Equal(t, block, retrieved)
	})
}

func TestFinalizeClusterBlock(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		parent := unittest.ClusterBlockFixture()
		block := unittest.ClusterBlockWithParent(&parent)

		err := db.Update(InsertClusterBlock(&block))
		require.NoError(t, err)

		err = db.Update(operation.IndexClusterBlockHeight(block.Header.ChainID, parent.Header.Height, parent.ID()))
		require.NoError(t, err)

		err = db.Update(operation.InsertClusterFinalizedHeight(block.Header.ChainID, parent.Header.Height))
		require.NoError(t, err)

		err = db.Update(FinalizeClusterBlock(block.Header.ID()))
		require.NoError(t, err)

		var boundary uint64
		err = db.View(operation.RetrieveClusterFinalizedHeight(block.Header.ChainID, &boundary))
		require.NoError(t, err)
		require.Equal(t, block.Header.Height, boundary)

		var headID flow.Identifier
		err = db.View(operation.LookupClusterBlockHeight(block.Header.ChainID, boundary, &headID))
		require.NoError(t, err)
		require.Equal(t, block.ID(), headID)
	})
}
