package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
)

func TestEventStoreRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewEvents(metrics, db)

		blockID := unittest.IdentifierFixture()
		tx1ID := unittest.IdentifierFixture()
		tx2ID := unittest.IdentifierFixture()
		evt1 := unittest.EventFixture(flow.EventAccountCreated, 0, 0, tx1ID)
		evt2 := unittest.EventFixture(flow.EventAccountCreated, 1, 1, tx2ID)
		evt3 := unittest.EventFixture(flow.EventAccountUpdated, 2, 2, tx2ID)
		expected := []flow.Event{
			evt1,
			evt2,
			evt3,
		}

		batch := badgerstorage.NewBatch(db)
		// store event
		err := store.BatchStore(blockID, expected, batch)
		require.NoError(t, err)

		err = batch.Flush()
		require.NoError(t, err)

		// retrieve by blockID
		actual, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, actual, 3)
		require.Contains(t, actual, evt1)
		require.Contains(t, actual, evt2)
		require.Contains(t, actual, evt3)

		// retrieve by blockID and event type
		actual, err = store.ByBlockIDEventType(blockID, flow.EventAccountCreated)
		require.NoError(t, err)
		require.Len(t, actual, 2)
		require.Contains(t, actual, evt1)
		require.Contains(t, actual, evt2)

		actual, err = store.ByBlockIDEventType(blockID, flow.EventAccountUpdated)
		require.NoError(t, err)
		require.Len(t, actual, 1)
		require.Contains(t, actual, evt3)

		actual, err = store.ByBlockIDEventType(blockID, flow.EventEpochSetup)
		require.NoError(t, err)
		require.Len(t, actual, 0)

		// retrieve by blockID and transaction id
		actual, err = store.ByBlockIDTransactionID(blockID, tx1ID)
		require.NoError(t, err)
		require.Len(t, actual, 1)
		require.Contains(t, actual, evt1)

		// test loading from database

		newStore := badgerstorage.NewEvents(metrics, db)
		actual, err = newStore.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, actual, 3)
		require.Contains(t, actual, evt1)
		require.Contains(t, actual, evt2)
		require.Contains(t, actual, evt3)
	})
}

func TestEventRetrieveWithoutStore(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewEvents(metrics, db)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()

		// retrieve by blockID
		events, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.True(t, len(events) == 0)

		// retrieve by blockID and event type
		events, err = store.ByBlockIDEventType(blockID, flow.EventAccountCreated)
		require.NoError(t, err)
		require.True(t, len(events) == 0)

		// retrieve by blockID and transaction id
		events, err = store.ByBlockIDTransactionID(blockID, txID)
		require.NoError(t, err)
		require.True(t, len(events) == 0)

	})
}
