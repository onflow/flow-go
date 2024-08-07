package operation

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlockChildrenIndexUpdateLookup(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		blockID := unittest.IdentifierFixture()
		childrenIDs := unittest.IdentifierListFixture(8)
		var retrievedIDs flow.IdentifierList

		err := InsertBlockChildren(blockID, childrenIDs)(db)
		require.NoError(t, err)
		err = RetrieveBlockChildren(blockID, &retrievedIDs)(db)
		require.NoError(t, err)
		assert.Equal(t, childrenIDs, retrievedIDs)

		altIDs := unittest.IdentifierListFixture(4)
		err = UpdateBlockChildren(blockID, altIDs)(db)
		require.NoError(t, err)
		err = RetrieveBlockChildren(blockID, &retrievedIDs)(db)
		require.NoError(t, err)
		assert.Equal(t, altIDs, retrievedIDs)
	})
}
