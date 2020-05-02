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

		err := db.Update(InsertBlock(block.ID(), &block))
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
		block := unittest.BlockWithParentFixture(parent.Header)

		err := db.Update(InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = db.Update(operation.IndexBlockHeight(parent.Header.Height, parent.ID()))
		require.NoError(t, err)

		err = db.Update(operation.InsertFinalizedHeight(parent.Header.Height))
		require.NoError(t, err)

		err = db.Update(FinalizeBlock(block.Header.ID()))
		require.NoError(t, err)

		var height uint64
		err = db.View(operation.RetrieveFinalizedHeight(&height))
		require.NoError(t, err)
		require.Equal(t, block.Header.Height, height)

		var headID flow.Identifier
		err = db.View(operation.LookupBlockHeight(height, &headID))
		require.NoError(t, err)
		require.Equal(t, block.ID(), headID)
	})
}
