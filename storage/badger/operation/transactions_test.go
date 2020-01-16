package operation

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTransactions(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.TransactionFixture()
		err := db.Update(InsertTransaction(&expected.TransactionBody))
		require.Nil(t, err)

		var actual flow.Transaction
		err = db.View(RetrieveTransaction(expected.ID(), &actual.TransactionBody))
		require.Nil(t, err)
		assert.Equal(t, expected, actual)

		err = db.Update(RemoveTransaction(expected.ID()))
		require.Nil(t, err)

		err = db.View(RetrieveTransaction(expected.ID(), &actual.TransactionBody))
		// should fail since this was just deleted
		if assert.Error(t, err) {
			assert.True(t, errors.Is(err, storage.ErrNotFound))
		}
	})
}
