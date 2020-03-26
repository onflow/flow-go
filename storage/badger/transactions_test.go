package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bstorage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTransactions(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := bstorage.NewTransactions(db)

		expected := unittest.TransactionFixture()
		err := store.Store(&expected.TransactionBody)
		require.Nil(t, err)

		actual, err := store.ByID(expected.ID())
		require.Nil(t, err)

		assert.Equal(t, &expected.TransactionBody, actual)
	})
}
