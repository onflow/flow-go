package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInsertProtocolState tests if basic badger operations on ProtocolState work as expected.
func TestInsertProtocolState(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.ProtocolStateFixture().ProtocolStateEntry

		protocolStateID := expected.ID()
		err := db.Update(InsertProtocolState(protocolStateID, expected))
		require.Nil(t, err)

		var actual flow.ProtocolStateEntry
		err = db.View(RetrieveProtocolState(protocolStateID, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)

		blockID := unittest.IdentifierFixture()
		err = db.Update(IndexProtocolState(blockID, protocolStateID))
		require.Nil(t, err)

		var actualProtocolStateID flow.Identifier
		err = db.View(LookupProtocolState(blockID, &actualProtocolStateID))
		require.Nil(t, err)

		assert.Equal(t, protocolStateID, actualProtocolStateID)
	})
}
