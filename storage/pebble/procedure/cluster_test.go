package procedure

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInsertRetrieveClusterBlock(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		block := unittest.ClusterBlockFixture()

		err := operation.WithReaderBatchWriter(db, InsertClusterBlock(&block))
		require.NoError(t, err)

		var retrieved cluster.Block
		err = RetrieveClusterBlock(block.Header.ID(), &retrieved)(db)
		require.NoError(t, err)

		require.Equal(t, block, retrieved)
	})
}

func TestFinalizeClusterBlock(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		parent := unittest.ClusterBlockFixture()

		block := unittest.ClusterBlockWithParent(&parent)

		err := operation.WithReaderBatchWriter(db, InsertClusterBlock(&block))
		require.NoError(t, err)

		rw := operation.NewPebbleReaderBatchWriter(db)
		_, tx := rw.ReaderWriter()
		err = operation.IndexClusterBlockHeight(block.Header.ChainID, parent.Header.Height, parent.ID())(tx)
		require.NoError(t, err)

		err = operation.InsertClusterFinalizedHeight(block.Header.ChainID, parent.Header.Height)(tx)
		require.NoError(t, err)
		require.NoError(t, rw.Commit())

		err = operation.WithReaderBatchWriter(db, FinalizeClusterBlock(block.Header.ID()))
		require.NoError(t, err)

		var boundary uint64
		err = operation.RetrieveClusterFinalizedHeight(block.Header.ChainID, &boundary)(db)
		require.NoError(t, err)
		require.Equal(t, block.Header.Height, boundary)

		var headID flow.Identifier
		err = operation.LookupClusterBlockHeight(block.Header.ChainID, boundary, &headID)(db)
		require.NoError(t, err)
		require.Equal(t, block.ID(), headID)
	})
}
