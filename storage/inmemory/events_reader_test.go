package inmemory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEvents_HappyPath(t *testing.T) {
	// Define test block and transaction
	blockID := unittest.IdentifierFixture()
	transaction1 := unittest.TransactionFixture()
	transaction2 := unittest.TransactionFixture()

	event1 := unittest.EventFixture(
		unittest.Event.WithEventType(flow.EventAccountCreated),
		unittest.Event.WithTransactionIndex(0),
		unittest.Event.WithEventIndex(0),
		unittest.Event.WithTransactionID(transaction1.ID()),
	)
	event2 := unittest.EventFixture(
		unittest.Event.WithEventType(flow.EventAccountUpdated),
		unittest.Event.WithTransactionIndex(0),
		unittest.Event.WithEventIndex(1),
		unittest.Event.WithTransactionID(transaction1.ID()),
	)
	event3 := unittest.EventFixture(
		unittest.Event.WithEventType(flow.EventAccountCreated),
		unittest.Event.WithTransactionIndex(1),
		unittest.Event.WithEventIndex(2),
		unittest.Event.WithTransactionID(transaction2.ID()),
	)

	expectedStoredEvents := []flow.Event{event1, event2, event3}

	// Store events
	eventsStore := NewEvents(blockID, expectedStoredEvents)

	// Retrieve events by block ID
	storedEvents, err := eventsStore.ByBlockID(blockID)
	require.NoError(t, err)
	assert.Len(t, storedEvents, len(expectedStoredEvents))
	assert.Contains(t, storedEvents, event1)
	assert.Contains(t, storedEvents, event2)
	assert.Contains(t, storedEvents, event3)

	// Retrieve events by transaction ID
	txEvents, err := eventsStore.ByBlockIDTransactionID(blockID, transaction1.ID())
	require.NoError(t, err)
	assert.Len(t, txEvents, 2)
	assert.Equal(t, event1, txEvents[0])
	assert.Equal(t, event2, txEvents[1])

	// Retrieve events by transaction index
	indexEvents, err := eventsStore.ByBlockIDTransactionIndex(blockID, 1)
	require.NoError(t, err)
	assert.Len(t, indexEvents, 1)
	assert.Equal(t, event3, indexEvents[0])

	// Retrieve events by event type
	typeEvents, err := eventsStore.ByBlockIDEventType(blockID, flow.EventAccountCreated)
	require.NoError(t, err)
	assert.Len(t, typeEvents, 2)
	assert.Contains(t, typeEvents, event1)
	assert.Contains(t, typeEvents, event3)
}
