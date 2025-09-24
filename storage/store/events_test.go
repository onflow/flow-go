package store_test

import (
	"math/rand"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEventStoreRetrieve(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		events := store.NewEvents(metrics, db)

		blockID := unittest.IdentifierFixture()
		tx1ID := unittest.IdentifierFixture()
		tx2ID := unittest.IdentifierFixture()
		evt1_1 := unittest.EventFixture(
			unittest.Event.WithEventType(flow.EventAccountCreated),
			unittest.Event.WithTransactionIndex(0),
			unittest.Event.WithEventIndex(0),
			unittest.Event.WithTransactionID(tx1ID),
		)
		evt1_2 := unittest.EventFixture(
			unittest.Event.WithEventType(flow.EventAccountCreated),
			unittest.Event.WithTransactionIndex(1),
			unittest.Event.WithEventIndex(1),
			unittest.Event.WithTransactionID(tx2ID),
		)

		evt2_1 := unittest.EventFixture(
			unittest.Event.WithEventType(flow.EventAccountUpdated),
			unittest.Event.WithTransactionIndex(2),
			unittest.Event.WithEventIndex(2),
			unittest.Event.WithTransactionID(tx2ID),
		)

		expected := []flow.EventsList{
			{evt1_1, evt1_2},
			{evt2_1},
		}

		unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				// store event
				return events.BatchStore(lctx, blockID, expected, rw)
			})
		})

		// retrieve by blockID
		actual, err := events.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, actual, 3)
		require.Contains(t, actual, evt1_1)
		require.Contains(t, actual, evt1_2)
		require.Contains(t, actual, evt2_1)

		// retrieve by blockID and event type
		actual, err = events.ByBlockIDEventType(blockID, flow.EventAccountCreated)
		require.NoError(t, err)
		require.Len(t, actual, 2)
		require.Contains(t, actual, evt1_1)
		require.Contains(t, actual, evt1_2)

		actual, err = events.ByBlockIDEventType(blockID, flow.EventAccountUpdated)
		require.NoError(t, err)
		require.Len(t, actual, 1)
		require.Contains(t, actual, evt2_1)

		evts := systemcontracts.ServiceEventsForChain(flow.Emulator)

		actual, err = events.ByBlockIDEventType(blockID, evts.EpochSetup.EventType())
		require.NoError(t, err)
		require.Len(t, actual, 0)

		// retrieve by blockID and transaction id
		actual, err = events.ByBlockIDTransactionID(blockID, tx1ID)
		require.NoError(t, err)
		require.Len(t, actual, 1)
		require.Contains(t, actual, evt1_1)

		// retrieve by blockID and transaction index
		actual, err = events.ByBlockIDTransactionIndex(blockID, 1)
		require.NoError(t, err)
		require.Len(t, actual, 1)
		require.Contains(t, actual, evt1_2)

		// test loading from database

		newStore := store.NewEvents(metrics, db)
		actual, err = newStore.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, actual, 3)
		require.Contains(t, actual, evt1_1)
		require.Contains(t, actual, evt1_2)
		require.Contains(t, actual, evt2_1)
	})
}

func TestEventRetrieveWithoutStore(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		events := store.NewEvents(metrics, db)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()
		txIndex := rand.Uint32()

		// retrieve by blockID
		evts, err := events.ByBlockID(blockID)
		require.NoError(t, err)
		require.True(t, len(evts) == 0)

		// retrieve by blockID and event type
		evts, err = events.ByBlockIDEventType(blockID, flow.EventAccountCreated)
		require.NoError(t, err)
		require.True(t, len(evts) == 0)

		// retrieve by blockID and transaction id
		evts, err = events.ByBlockIDTransactionID(blockID, txID)
		require.NoError(t, err)
		require.True(t, len(evts) == 0)

		// retrieve by blockID and transaction id
		evts, err = events.ByBlockIDTransactionIndex(blockID, txIndex)
		require.NoError(t, err)
		require.True(t, len(evts) == 0)

	})
}

func TestEventStoreAndRemove(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := store.NewEvents(metrics, db)

		// Create and store an event
		blockID := unittest.IdentifierFixture()
		tx1ID := unittest.IdentifierFixture()
		tx2ID := unittest.IdentifierFixture()
		evt1_1 := unittest.EventFixture(
			unittest.Event.WithEventType(flow.EventAccountCreated),
			unittest.Event.WithTransactionIndex(0),
			unittest.Event.WithEventIndex(0),
			unittest.Event.WithTransactionID(tx1ID),
		)
		evt1_2 := unittest.EventFixture(
			unittest.Event.WithEventType(flow.EventAccountCreated),
			unittest.Event.WithTransactionIndex(1),
			unittest.Event.WithEventIndex(1),
			unittest.Event.WithTransactionID(tx2ID),
		)

		evt2_1 := unittest.EventFixture(
			unittest.Event.WithEventType(flow.EventAccountUpdated),
			unittest.Event.WithTransactionIndex(2),
			unittest.Event.WithEventIndex(2),
			unittest.Event.WithTransactionID(tx2ID),
		)

		expected := []flow.EventsList{
			{evt1_1, evt1_2},
			{evt2_1},
		}

		unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
			return store.Store(lctx, blockID, expected)
		})

		// Ensure it exists
		event, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, event, 3)
		require.Contains(t, event, evt1_1)
		require.Contains(t, event, evt1_2)
		require.Contains(t, event, evt2_1)

		// Remove it
		err = store.RemoveByBlockID(blockID)
		require.NoError(t, err)

		// Ensure it no longer exists
		event, err = store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, event, 0)
	})
}
