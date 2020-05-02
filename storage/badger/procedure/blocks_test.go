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

		// This will break test
		//block.ChainID = "\x89krg\u007fBN\x1d\xf5\xfb\xb8r\xbc4\xbd\x98ռ\xf1\xd0twU\xbf\x16N\xb4?,\xa0&;"

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
		block := unittest.BlockWithParentFixture(parent.Header)

		err := db.Update(InsertBlock(&block))
		require.NoError(t, err)

		err = db.Update(operation.InsertNumber(parent.Header.Height, parent.ID()))
		require.NoError(t, err)

		err = db.Update(operation.InsertBoundary(parent.Header.Height))
		require.NoError(t, err)

		err = db.Update(FinalizeBlock(block.Header.ID()))
		require.NoError(t, err)

		var boundary uint64
		err = db.View(operation.RetrieveBoundary(&boundary))
		require.NoError(t, err)
		require.Equal(t, block.Header.Height, boundary)

		var headID flow.Identifier
		err = db.View(operation.RetrieveNumber(boundary, &headID))
		require.NoError(t, err)
		require.Equal(t, block.ID(), headID)
	})
}

func TestInsertRetrieveBlockByCollectionGuarantee(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		block := unittest.BlockFixture()

		err := db.Update(InsertBlock(&block))
		require.NoError(t, err)

		err = db.Update(IndexBlockByGuarantees(block.ID()))
		require.NoError(t, err)

		var retrieved flow.Block
		for _, g := range block.Payload.Guarantees {
			collID := g.CollectionID
			err = db.View(RetrieveBlockByCollectionID(collID, &retrieved))
			require.NoError(t, err)
			require.Equal(t, block, retrieved)
		}
	})
}
