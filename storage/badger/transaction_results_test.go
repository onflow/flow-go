package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	bstorage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTransactionResults(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := bstorage.NewTransactionResults(db)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()
		expected := &flow.TransactionResult{
			TransactionID: txID,
			ErrorMessage:  "a runtime error",
		}
		err := store.Store(blockID, expected)
		require.Nil(t, err)

		actual, err := store.ByBlockIDTransactionID(blockID, txID)
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}
