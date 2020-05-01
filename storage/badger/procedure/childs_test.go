package procedure_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestIndexAndRetrieveChild(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		block1 := &flow.Header{
			ParentID: flow.ZeroID,
			View:     1,
		}
		block2 := &flow.Header{
			ParentID: block1.ID(),
			View:     2,
		}

		// store parent
		headersDB := storage.NewHeaders(db)
		err := headersDB.Store(block1)

		// stored headers can be retrieved
		var childHeader flow.Header
		err = db.View(operation.RetrieveHeader(block1.ID(), &childHeader))
		require.NoError(t, err)

		// store child
		err = headersDB.Store(block2)
		require.NoError(t, err)

		parentID, childID := block1.ID(), block2.ID()

		err = db.Update(procedure.IndexChildByBlockID(parentID, childID))
		require.NoError(t, err)

		// retrieve child
		var retrievedChild flow.Header
		err = db.View(procedure.RetrieveChildByBlockID(parentID, &retrievedChild))
		require.NoError(t, err)

		// retrieved child should be the stored child
		require.Equal(t, childID, retrievedChild.ID())
	})
}
