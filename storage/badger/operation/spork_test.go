package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestSporkRootBlock_InsertRetrieve verifies that a spork root block can be
// correctly inserted into and retrieved from the Badger database.
func TestSporkRootBlockID_InsertRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		sporkRootBlockID := unittest.IdentifierFixture()

		err := db.Update(IndexSporkRootBlock(sporkRootBlockID))
		require.NoError(t, err)

		var actual flow.Identifier
		err = db.View(RetrieveSporkRootBlockID(&actual))
		require.NoError(t, err)

		assert.Equal(t, sporkRootBlockID, actual)
	})
}
