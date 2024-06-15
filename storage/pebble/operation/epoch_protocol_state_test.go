package operation

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInsertProtocolState tests if basic pebble operations on EpochProtocolState work as expected.
func TestInsertEpochProtocolState(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		expected := unittest.EpochStateFixture().EpochProtocolStateEntry

		epochProtocolStateEntryID := expected.ID()
		err := InsertEpochProtocolState(epochProtocolStateEntryID, expected)(db)
		require.Nil(t, err)

		var actual flow.EpochProtocolStateEntry
		err = RetrieveEpochProtocolState(epochProtocolStateEntryID, &actual)(db)
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)

		blockID := unittest.IdentifierFixture()
		err = IndexEpochProtocolState(blockID, epochProtocolStateEntryID)(db)
		require.Nil(t, err)

		var actualProtocolStateID flow.Identifier
		err = LookupEpochProtocolState(blockID, &actualProtocolStateID)(db)
		require.Nil(t, err)

		assert.Equal(t, epochProtocolStateEntryID, actualProtocolStateID)
	})
}
