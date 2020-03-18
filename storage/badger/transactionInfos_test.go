package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bstorage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTransactionInfos(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := bstorage.NewTransactionInfo(db)

		expected := unittest.TransactionInfoFixture()
		err := store.Store(&expected)
		require.Nil(t, err)

		actual, err := store.ByID(expected.ID())
		require.Nil(t, err)

		assert.Equal(t, &expected, actual)
	})
}
