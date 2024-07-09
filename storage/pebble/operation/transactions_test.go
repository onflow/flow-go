package operation

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactions(t *testing.T) {

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		expected := unittest.TransactionFixture()
		err := InsertTransaction(expected.ID(), &expected.TransactionBody)(db)
		require.Nil(t, err)

		var actual flow.Transaction
		err = RetrieveTransaction(expected.ID(), &actual.TransactionBody)(db)
		require.Nil(t, err)
		assert.Equal(t, expected, actual)
	})
}
