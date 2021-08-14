package operation

import (
	"bytes"
	"sort"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestRetrieveEventByBlockIDTxID tests event insertion, event retrieval by block id, block id and transaction id,
// and block id and event type
func TestRetrieveEventByBlockIDTxID(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		// create block ids, transaction ids and event types slices
		blockIDs := []flow.Identifier{flow.HashToID([]byte{0x01}), flow.HashToID([]byte{0x02})}
		txIDs := []flow.Identifier{flow.HashToID([]byte{0x11}), flow.HashToID([]byte{0x12})}
		eTypes := []flow.EventType{flow.EventAccountCreated, flow.EventAccountUpdated}

		// create map of block id to event, tx id to event and event type to event
		blockMap := make(map[string][]flow.Event)
		txMap := make(map[string][]flow.Event)
		typeMap := make(map[string][]flow.Event)

		// initialize the maps and the db
		for _, b := range blockIDs {

			bEvents := make([]flow.Event, 0)

			// all blocks share the same transactions
			for i, tx := range txIDs {

				tEvents := make([]flow.Event, 0)

				// create one event for each possible event type
				for j, etype := range eTypes {

					eEvents := make([]flow.Event, 0)

					event := unittest.EventFixture(etype, uint32(i), uint32(j), tx, 0)

					// insert event into the db
					err := db.Update(InsertEvent(b, event))
					require.Nil(t, err)

					// update event arrays in the maps
					bEvents = append(bEvents, event)
					tEvents = append(tEvents, event)
					eEvents = append(eEvents, event)

					key := b.String() + "_" + string(etype)
					if _, ok := typeMap[key]; ok {
						typeMap[key] = append(typeMap[key], eEvents...)
					} else {
						typeMap[key] = eEvents
					}
				}
				txMap[b.String()+"_"+tx.String()] = tEvents
			}
			blockMap[b.String()] = bEvents
		}

		assertFunc := func(err error, expected []flow.Event, actual []flow.Event) {
			require.NoError(t, err)
			sortEvent(expected)
			sortEvent(actual)
			require.Equal(t, expected, actual)
		}

		t.Run("retrieve events by Block ID", func(t *testing.T) {
			for _, b := range blockIDs {
				var actualEvents = make([]flow.Event, 0)

				// lookup events by block id
				err := db.View(LookupEventsByBlockID(b, &actualEvents))

				expectedEvents := blockMap[b.String()]
				assertFunc(err, expectedEvents, actualEvents)
			}
		})

		t.Run("retrieve events by block ID and transaction ID", func(t *testing.T) {
			for _, b := range blockIDs {
				for _, t := range txIDs {
					var actualEvents = make([]flow.Event, 0)

					//lookup events by block id and transaction id
					err := db.View(RetrieveEvents(b, t, &actualEvents))

					expectedEvents := txMap[b.String()+"_"+t.String()]
					assertFunc(err, expectedEvents, actualEvents)
				}
			}
		})

		t.Run("retrieve events by block ID and event type", func(t *testing.T) {
			for _, b := range blockIDs {
				for _, et := range eTypes {
					var actualEvents = make([]flow.Event, 0)

					//lookup events by block id and transaction id
					err := db.View(LookupEventsByBlockIDEventType(b, et, &actualEvents))

					expectedEvents := typeMap[b.String()+"_"+string(et)]
					assertFunc(err, expectedEvents, actualEvents)
				}
			}
		})
	})
}

// Event retrieval does not guarantee any order, hence a sort function to help compare expected and actual events
func sortEvent(events []flow.Event) {
	sort.Slice(events, func(i, j int) bool {

		tComp := bytes.Compare(events[i].TransactionID[:], events[j].TransactionID[:])
		if tComp < 0 {
			return true
		}
		if tComp > 0 {
			return false
		}

		txIndex := events[i].TransactionIndex == events[j].TransactionIndex
		if !txIndex {
			return events[i].TransactionIndex < events[j].TransactionIndex
		}

		return events[i].EventIndex < events[j].EventIndex

	})
}
