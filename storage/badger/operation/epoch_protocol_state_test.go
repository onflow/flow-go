package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInsertProtocolState tests if basic badger operations on EpochProtocolState work as expected.
func TestInsertEpochProtocolState(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.EpochStateFixture().EpochProtocolStateEntry

		epochProtocolStateEntryID := expected.ID()
		err := db.Update(InsertEpochProtocolState(epochProtocolStateEntryID, expected))
		require.Nil(t, err)

		var actual flow.EpochProtocolStateEntry
		err = db.View(RetrieveEpochProtocolState(epochProtocolStateEntryID, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)

		blockID := unittest.IdentifierFixture()
		err = db.Update(IndexEpochProtocolState(blockID, epochProtocolStateEntryID))
		require.Nil(t, err)

		var actualProtocolStateID flow.Identifier
		err = db.View(LookupEpochProtocolState(blockID, &actualProtocolStateID))
		require.Nil(t, err)

		assert.Equal(t, epochProtocolStateEntryID, actualProtocolStateID)
	})
}
