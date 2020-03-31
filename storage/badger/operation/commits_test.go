package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestStateCommitments(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(t *testing.T, db *badger.DB) {
		expected := unittest.StateCommitmentFixture()
		id := unittest.IdentifierFixture()
		err := db.Update(IndexCommit(id, expected))
		require.Nil(t, err)

		var actual flow.StateCommitment
		err = db.View(LookupCommit(id, &actual))
		require.Nil(t, err)
		assert.Equal(t, expected, actual)
	})
}
