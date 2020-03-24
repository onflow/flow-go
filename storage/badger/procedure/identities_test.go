package procedure

import (
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestInsertIndexRetreiveIdentities(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		block := unittest.BlockFixture()

		err := db.Update(operation.InsertHeader(&block.Header))
		require.NoError(t, err)

		err = db.Update(func(tx *badger.Txn) error {
			for _, identity := range block.Identities {
				err := operation.InsertIdentity(identity)(tx)
				if err != nil {
					return fmt.Errorf("could not insert identity (%x): %w", identity.NodeID, err)
				}
			}
			return nil
		})
		require.NoError(t, err)

		err = db.Update(IndexIdentities(block.Height, block.ID(), block.ParentID, block.Identities))
		require.NoError(t, err)

		var retrieved []*flow.Identity
		err = db.View(RetrieveIdentities(block.ID(), &retrieved))
		require.NoError(t, err)

		for i, id := range block.Identities {
			require.Equal(t, id, retrieved[i])
		}
	})
}
