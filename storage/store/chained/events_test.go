package chained

import (
	"sort"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEventsOnlyFirstHave(t *testing.T) {
	unittest.RunWithBadgerDBAndPebbleDB(t, func(bdb *badger.DB, pdb *pebble.DB) {
		bevents := store.NewEvents(metrics.NewNoopCollector(), badgerimpl.ToDB(bdb))
		pevents := store.NewEvents(metrics.NewNoopCollector(), pebbleimpl.ToDB(pdb))

		blockID := unittest.IdentifierFixture()
		events := unittest.EventsFixture(3)

		chained := NewEvents(pevents, bevents)

		// not found
		actual, err := chained.ByBlockID(blockID)
		require.NoError(t, err)
		require.Len(t, actual, 0)

		// only stored in first
		require.NoError(t, pevents.Store(blockID, []flow.EventsList{events}))
		actual, err = chained.ByBlockID(blockID)
		require.NoError(t, err)

		eventsEqual(t, events, actual)
	})
}

func TestEventsOnlySecondHave(t *testing.T) {
	unittest.RunWithBadgerDBAndPebbleDB(t, func(bdb *badger.DB, pdb *pebble.DB) {
		bevents := store.NewEvents(metrics.NewNoopCollector(), badgerimpl.ToDB(bdb))
		pevents := store.NewEvents(metrics.NewNoopCollector(), pebbleimpl.ToDB(pdb))

		blockID := unittest.IdentifierFixture()
		events := unittest.EventsFixture(3)

		chained := NewEvents(pevents, bevents)
		// only stored in second
		require.NoError(t, bevents.Store(blockID, []flow.EventsList{events}))
		actual, err := chained.ByBlockID(blockID)
		require.NoError(t, err)

		eventsEqual(t, events, actual)
	})
}

func eventsEqual(t *testing.T, expected, actual []flow.Event) {
	require.Len(t, actual, len(expected)) // Ensure they have the same length

	sortEvents(expected)
	sortEvents(actual)

	require.Equal(t, expected, actual)
}

// Define a sorting method based on event properties
func sortEvents(events []flow.Event) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].EventIndex < events[j].EventIndex
	})
}
