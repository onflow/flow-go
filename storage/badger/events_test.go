package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
)

func TestEventStoreRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := badgerstorage.NewEvents(db)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()
		expected := []flow.Event{unittest.EventFixture(flow.EventAccountCreated, 0, 0, txID)}

		// store event
		err := store.Store(blockID, expected)
		require.NoError(t, err)

		// retrieve by blockID
		actual, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)

		// retrieve by blockID and event type
		actual, err = store.ByBlockIDEventType(blockID, flow.EventAccountCreated)
		require.NoError(t, err)
		require.Equal(t, expected, actual)

		// retrieve by blockID and transaction id
		actual, err = store.ByBlockIDTransactionID(blockID, txID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}
