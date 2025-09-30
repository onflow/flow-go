package unsynchronized

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEvents_HappyPath(t *testing.T) {
	// Create an instance of Events
	eventsStore := NewEvents()

	// Define test block and transaction
	block := unittest.BlockFixture()
	transaction1 := unittest.TransactionFixture()
	transaction2 := unittest.TransactionFixture()

	event1 := unittest.EventFixture(
		unittest.Event.WithEventType(flow.EventAccountCreated),
		unittest.Event.WithTransactionIndex(0),
		unittest.Event.WithEventIndex(0),
		unittest.Event.WithTransactionID(transaction1.Hash()),
	)
	event2 := unittest.EventFixture(
		unittest.Event.WithEventType(flow.EventAccountUpdated),
		unittest.Event.WithTransactionIndex(0),
		unittest.Event.WithEventIndex(1),
		unittest.Event.WithTransactionID(transaction1.Hash()),
	)
	event3 := unittest.EventFixture(
		unittest.Event.WithEventType(flow.EventAccountCreated),
		unittest.Event.WithTransactionIndex(1),
		unittest.Event.WithEventIndex(2),
		unittest.Event.WithTransactionID(transaction2.Hash()),
	)

	// Store events
	expectedStoredEvents := flow.EventsList{event1, event2, event3}
	err := eventsStore.Store(block.Hash(), []flow.EventsList{expectedStoredEvents})
	require.NoError(t, err)

	// Retrieve events by block ID
	storedEvents, err := eventsStore.ByBlockID(block.Hash())
	require.NoError(t, err)
	assert.Len(t, storedEvents, len(expectedStoredEvents))
	assert.Contains(t, storedEvents, event1)
	assert.Contains(t, storedEvents, event2)
	assert.Contains(t, storedEvents, event3)

	// Retrieve events by transaction ID
	txEvents, err := eventsStore.ByBlockIDTransactionID(block.Hash(), transaction1.Hash())
	require.NoError(t, err)
	assert.Len(t, txEvents, 2)
	assert.Equal(t, event1, txEvents[0])
	assert.Equal(t, event2, txEvents[1])

	// Retrieve events by transaction index
	indexEvents, err := eventsStore.ByBlockIDTransactionIndex(block.Hash(), 1)
	require.NoError(t, err)
	assert.Len(t, indexEvents, 1)
	assert.Equal(t, event3, indexEvents[0])

	// Retrieve events by event type
	typeEvents, err := eventsStore.ByBlockIDEventType(block.Hash(), flow.EventAccountCreated)
	require.NoError(t, err)
	assert.Len(t, typeEvents, 2)
	assert.Contains(t, typeEvents, event1)
	assert.Contains(t, typeEvents, event3)

	// Extract structured data
	events := eventsStore.Data()
	require.Len(t, events, len(expectedStoredEvents))
	require.ElementsMatch(t, events, expectedStoredEvents)
}
