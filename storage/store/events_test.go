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

		err := unittest.WithLock(t, lockManager, storage.LockInsertEvent, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				// store event
				return events.BatchStore(lctx, blockID, expected, rw)
			})
		})
		require.NoError(t, err)

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

// TestByBlockID_SortsEventsByExecutionOrder verifies that events loaded from the
// database (cache miss path) are returned sorted by (TransactionIndex,
// EventIndex) regardless of the order imposed by the storage key, which is
// [blockID, txID, txIndex, eventIndex].  When two transactions have txIDs whose
// byte order differs from their execution order the DB iteration would return
// them in txID order; the retrieve function must re-sort them.
func TestByBlockID_SortsEventsByExecutionOrder(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		m := metrics.NewNoopCollector()
		eventsStore := store.NewEvents(m, db)

		blockID := unittest.IdentifierFixture()

		// txIDLow sorts before txIDHigh in byte (DB key) order, but its
		// TransactionIndex is higher, so in execution order it comes second.
		var txIDLow, txIDHigh flow.Identifier
		txIDLow[0] = 0x00  // first in DB key order
		txIDHigh[0] = 0xFF // second in DB key order

		// execution tx 0 uses txIDHigh; execution tx 1 uses txIDLow
		evtTx0 := unittest.EventFixture(
			unittest.Event.WithTransactionID(txIDHigh),
			unittest.Event.WithTransactionIndex(0),
			unittest.Event.WithEventIndex(0),
		)
		evtTx1A := unittest.EventFixture(
			unittest.Event.WithTransactionID(txIDLow),
			unittest.Event.WithTransactionIndex(1),
			unittest.Event.WithEventIndex(0),
		)
		evtTx1B := unittest.EventFixture(
			unittest.Event.WithTransactionID(txIDLow),
			unittest.Event.WithTransactionIndex(1),
			unittest.Event.WithEventIndex(1),
		)

		err := unittest.WithLock(t, lockManager, storage.LockInsertEvent, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return eventsStore.BatchStore(lctx, blockID, []flow.EventsList{{evtTx0}, {evtTx1A, evtTx1B}}, rw)
			})
		})
		require.NoError(t, err)

		// Use a fresh store to force a DB load (bypasses the cache populated by BatchStore).
		freshStore := store.NewEvents(m, db)
		got, err := freshStore.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, got, 3)

		// Events must come back in execution order: txIndex 0, then 1 (eventIndex 0, 1).
		require.Equal(t, uint32(0), got[0].TransactionIndex)
		require.Equal(t, uint32(0), got[0].EventIndex)
		require.Equal(t, uint32(1), got[1].TransactionIndex)
		require.Equal(t, uint32(0), got[1].EventIndex)
		require.Equal(t, uint32(1), got[2].TransactionIndex)
		require.Equal(t, uint32(1), got[2].EventIndex)
	})
}

// TestServiceEventStoreRetrieve verifies basic store and retrieval for ServiceEvents.
func TestServiceEventStoreRetrieve(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		m := metrics.NewNoopCollector()
		svcEventsStore := store.NewServiceEvents(m, db)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()

		evt1 := unittest.EventFixture(
			unittest.Event.WithTransactionID(txID),
			unittest.Event.WithTransactionIndex(0),
			unittest.Event.WithEventIndex(0),
		)
		evt2 := unittest.EventFixture(
			unittest.Event.WithTransactionID(txID),
			unittest.Event.WithTransactionIndex(1),
			unittest.Event.WithEventIndex(0),
		)

		err := unittest.WithLock(t, lockManager, storage.LockInsertServiceEvent, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return svcEventsStore.BatchStore(lctx, blockID, []flow.Event{evt1, evt2}, rw)
			})
		})
		require.NoError(t, err)

		got, err := svcEventsStore.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, got, 2)
		require.Contains(t, got, evt1)
		require.Contains(t, got, evt2)

		// Test loading from database (cache miss path).
		freshStore := store.NewServiceEvents(m, db)
		got, err = freshStore.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, got, 2)
		require.Contains(t, got, evt1)
		require.Contains(t, got, evt2)
	})
}

// TestServiceEventByBlockID_SortsEventsByExecutionOrder verifies that service events
// loaded from the database (cache miss path) are returned sorted by (TransactionIndex,
// EventIndex) regardless of the DB key order, which is keyed by txID byte order.
// Before the fix, the sort was missing for ServiceEvents (unlike Events which already
// had it), causing events to be returned in DB key order on a cache miss.
func TestServiceEventByBlockID_SortsEventsByExecutionOrder(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		m := metrics.NewNoopCollector()
		svcEventsStore := store.NewServiceEvents(m, db)

		blockID := unittest.IdentifierFixture()

		// txIDLow sorts before txIDHigh in byte (DB key) order, but its
		// TransactionIndex is higher, so in execution order it comes second.
		var txIDLow, txIDHigh flow.Identifier
		txIDLow[0] = 0x00  // first in DB key order
		txIDHigh[0] = 0xFF // second in DB key order

		// execution tx 0 uses txIDHigh; execution tx 1 uses txIDLow
		evtTx0 := unittest.EventFixture(
			unittest.Event.WithTransactionID(txIDHigh),
			unittest.Event.WithTransactionIndex(0),
			unittest.Event.WithEventIndex(0),
		)
		evtTx1A := unittest.EventFixture(
			unittest.Event.WithTransactionID(txIDLow),
			unittest.Event.WithTransactionIndex(1),
			unittest.Event.WithEventIndex(0),
		)
		evtTx1B := unittest.EventFixture(
			unittest.Event.WithTransactionID(txIDLow),
			unittest.Event.WithTransactionIndex(1),
			unittest.Event.WithEventIndex(1),
		)

		err := unittest.WithLock(t, lockManager, storage.LockInsertServiceEvent, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return svcEventsStore.BatchStore(lctx, blockID, []flow.Event{evtTx0, evtTx1A, evtTx1B}, rw)
			})
		})
		require.NoError(t, err)

		// Use a fresh store to force a DB load (bypasses the cache populated by BatchStore).
		freshStore := store.NewServiceEvents(m, db)
		got, err := freshStore.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, got, 3)

		// Events must come back in execution order: txIndex 0, then 1 (eventIndex 0, 1).
		require.Equal(t, uint32(0), got[0].TransactionIndex)
		require.Equal(t, uint32(0), got[0].EventIndex)
		require.Equal(t, uint32(1), got[1].TransactionIndex)
		require.Equal(t, uint32(0), got[1].EventIndex)
		require.Equal(t, uint32(1), got[2].TransactionIndex)
		require.Equal(t, uint32(1), got[2].EventIndex)
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

		err := unittest.WithLock(t, lockManager, storage.LockInsertEvent, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store.BatchStore(lctx, blockID, expected, rw)
			})
		})
		require.NoError(t, err)

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
