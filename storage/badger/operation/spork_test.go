package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSporkID_InsertRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		sporkID := unittest.IdentifierFixture()

		err := db.Update(InsertSporkID(sporkID))
		require.NoError(t, err)

		var actual flow.Identifier
		err = db.View(RetrieveSporkID(&actual))
		require.NoError(t, err)

		assert.Equal(t, sporkID, actual)
	})
}
