package procedure

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInsertRetrieveIndex(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		blockID := unittest.IdentifierFixture()
		index := unittest.IndexFixture()

		err := db.Update(InsertIndex(blockID, index))
		require.NoError(t, err)

		var retrieved flow.Index
		err = db.View(RetrieveIndex(blockID, &retrieved))
		require.NoError(t, err)

		require.Equal(t, index, &retrieved)
	})
}
