package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTransactionsInsertRetrieve(t *testing.T) {

	unittest.RunWithDB(t, func(db *badger.DB) {
		expected := unittest.TransactionFixture()
		err := db.Update(InsertTransaction(expected.Hash(), &expected))
		require.Nil(t, err)

		var actual flow.Transaction
		err = db.View(RetrieveTransaction(expected.Hash(), &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}
