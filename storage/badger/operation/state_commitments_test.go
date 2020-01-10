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

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.StateCommitmentFixture()
		hash := unittest.HashFixture(5)
		err := db.Update(PersistStateCommitment(hash, &expected))
		require.Nil(t, err)

		var actual flow.StateCommitment
		err = db.View(RetrieveStateCommitment(hash, &actual))
		require.Nil(t, err)
		assert.Equal(t, expected, actual)
	})
}
