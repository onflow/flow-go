package procedure

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestInsertRetrieveBlock(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		block := unittest.BlockFixture()

		err := db.Update(InsertBlock(&block))
		require.NoError(t, err)

		var retrieved flow.Block
		err = db.View(RetrieveBlock(block.Header.ID(), &retrieved))
		require.NoError(t, err)

		require.Equal(t, block, retrieved)
	})
}

func TestFinalizeBlock(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		parent := unittest.BlockFixture()
		block := unittest.BlockWithParent(&parent)

		err := db.Update(InsertBlock(&block))
		require.NoError(t, err)

		err = db.Update(operation.InsertNumber(parent.View, parent.ID()))
		require.NoError(t, err)

		err = db.Update(operation.InsertBoundary(parent.View))
		require.NoError(t, err)

		err = db.Update(FinalizeBlock(block.Header.ID()))
		require.NoError(t, err)

		var boundary uint64
		err = db.View(operation.RetrieveBoundary(&boundary))
		require.NoError(t, err)
		require.Equal(t, block.View, boundary)

		var headID flow.Identifier
		err = db.View(operation.RetrieveNumber(boundary, &headID))
		require.NoError(t, err)
		require.Equal(t, block.ID(), headID)
	})
}
