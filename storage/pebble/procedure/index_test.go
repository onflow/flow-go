package procedure

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInsertRetrieveIndex(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		blockID := unittest.IdentifierFixture()
		index := unittest.IndexFixture()

		err := InsertIndex(blockID, index)(db)
		require.NoError(t, err)

		var retrieved flow.Index
		err = RetrieveIndex(blockID, &retrieved)(db)
		require.NoError(t, err)

		require.Equal(t, index, &retrieved)
	})
}
