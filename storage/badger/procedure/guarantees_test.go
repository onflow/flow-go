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

func TestInsertIndexRetreiveGuarantees(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		block := unittest.BlockFixture()

		err := db.Update(operation.InsertHeader(&block.Header))
		require.NoError(t, err)

		err = db.Update(func(tx *badger.Txn) error {
			for _, guarantee := range block.Guarantees {
				err := operation.InsertGuarantee(guarantee)(tx)
				if err != nil {
					return fmt.Errorf("could not insert guarantee (%x): %w", guarantee.CollectionID, err)
				}
			}
			return nil
		})
		require.NoError(t, err)

		err = db.Update(IndexGuarantees(block.Height, block.ID(), block.ParentID, block.Guarantees))
		require.NoError(t, err)

		var retrieved []*flow.CollectionGuarantee
		err = db.View(RetrieveGuarantees(block.ID(), &retrieved))
		require.NoError(t, err)

		for i, id := range block.Guarantees {
			require.Equal(t, id, retrieved[i])
		}
	})
}
