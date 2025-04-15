package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactions(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.TransactionFixture()
		err := db.Update(InsertTransaction(expected.ID(), &expected.TransactionBody))
		require.NoError(t, err)

		var actual flow.Transaction
		err = db.View(RetrieveTransaction(expected.ID(), &actual.TransactionBody))
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})
}
