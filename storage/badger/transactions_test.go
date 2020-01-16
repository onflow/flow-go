package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage"
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

		err = store.Remove(expected.ID())
		require.NoError(t, err)

		// should fail since this was just deleted
		_, err = store.ByID(expected.ID())
		if assert.Error(t, err) {
			assert.True(t, errors.Is(err, storage.ErrNotFound))
		}
	})
}
