package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestStateCommitments(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.StateCommitmentFixture()
		id := unittest.IdentifierFixture()
		err := db.Update(IndexStateCommitment(id, expected))
		require.Nil(t, err)

		var actual flow.StateCommitment
		err = db.View(LookupStateCommitment(id, &actual))
		require.Nil(t, err)
		assert.Equal(t, expected, actual)
	})
}
