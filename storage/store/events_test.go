package store_test

import (
	"math/rand"
	"sort"
	"sync"
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

// TestByBlockIDReturnsCopy verifies that ByBlockID returns a copy of the cached
// slice. Mutating the returned slice must not affect subsequent calls.
//
// Without this guarantee, callers that sort the returned slice (such as
// EventsIndex.ByBlockID) corrupt the cached ordering for every future reader of
// the same block.
func TestByBlockIDReturnsCopy(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		m := metrics.NewNoopCollector()
		eventsStore := store.NewEvents(m, db)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()

		// Store three events with EventIndex 0, 1, 2 in that order.
		evts := flow.EventsList{
			unittest.EventFixture(
				unittest.Event.WithTransactionID(txID),
				unittest.Event.WithTransactionIndex(0),
				unittest.Event.WithEventIndex(0),
				unittest.Event.WithPayload([]byte{1, 2, 3}),
			),
			unittest.EventFixture(
				unittest.Event.WithTransactionID(txID),
				unittest.Event.WithTransactionIndex(0),
				unittest.Event.WithEventIndex(1),
			),
			unittest.EventFixture(
				unittest.Event.WithTransactionID(txID),
				unittest.Event.WithTransactionIndex(0),
				unittest.Event.WithEventIndex(2),
			),
		}

		err := unittest.WithLock(t, lockManager, storage.LockInsertEvent, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return eventsStore.BatchStore(lctx, blockID, []flow.EventsList{evts}, rw)
			})
		})
		require.NoError(t, err)

		// First retrieval.
		got1, err := eventsStore.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, got1, 3)

		// Mutate the payload of the first event, and make sure the original is not modified.
		got1[0].Payload[0] = 3
		require.Equal(t, []byte{3, 2, 3}, got1[0].Payload)

		// Mutate the returned slice in-place (sort descending by EventIndex),
		// simulating what EventsIndex.ByBlockID does.
		sort.Slice(got1, func(i, j int) bool {
			return got1[i].EventIndex > got1[j].EventIndex
		})
		require.Equal(t, uint32(2), got1[0].EventIndex) // sanity: slice is reversed

		// Second retrieval must return events in the original stored order,
		// unaffected by the mutation above.
		got2, err := eventsStore.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, got2, 3)
		require.Equal(t, uint32(0), got2[0].EventIndex)
		require.Equal(t, uint32(1), got2[1].EventIndex)
		require.Equal(t, uint32(2), got2[2].EventIndex)

		// make sure the payload of the first event is not modified
		require.Equal(t, []byte{1, 2, 3}, got2[0].Payload)
	})
}

// TestByBlockIDConcurrentMutationIsRaceFree verifies there is no data race when
// multiple goroutines retrieve and sort the events for the same block
// concurrently. Run with -race to exercise the detector.
func TestByBlockIDConcurrentMutationIsRaceFree(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		m := metrics.NewNoopCollector()
		eventsStore := store.NewEvents(m, db)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()

		evts := flow.EventsList{
			unittest.EventFixture(
				unittest.Event.WithTransactionID(txID),
				unittest.Event.WithTransactionIndex(0),
				unittest.Event.WithEventIndex(0),
			),
			unittest.EventFixture(
				unittest.Event.WithTransactionID(txID),
				unittest.Event.WithTransactionIndex(0),
				unittest.Event.WithEventIndex(1),
			),
			unittest.EventFixture(
				unittest.Event.WithTransactionID(txID),
				unittest.Event.WithTransactionIndex(0),
				unittest.Event.WithEventIndex(2),
			),
		}

		err := unittest.WithLock(t, lockManager, storage.LockInsertEvent, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return eventsStore.BatchStore(lctx, blockID, []flow.EventsList{evts}, rw)
			})
		})
		require.NoError(t, err)

		var wg sync.WaitGroup
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				got, err := eventsStore.ByBlockID(blockID)
				if err != nil {
					return
				}
				// Simulate the in-place sort performed by EventsIndex.ByBlockID.
				sort.Slice(got, func(i, j int) bool {
					if got[i].TransactionIndex == got[j].TransactionIndex {
						return got[i].EventIndex < got[j].EventIndex
					}
					return got[i].TransactionIndex < got[j].TransactionIndex
				})
			}()
		}
		wg.Wait()
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
