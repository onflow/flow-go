package operation

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestStateCommitments(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		expected := unittest.StateCommitmentFixture()
		id := unittest.IdentifierFixture()
		err := IndexStateCommitment(id, expected)(db)
		require.Nil(t, err)

		var actual flow.StateCommitment
		err = LookupStateCommitment(id, &actual)(db)
		require.Nil(t, err)
		assert.Equal(t, expected, actual)
	})
}
