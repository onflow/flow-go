package unsynchronized

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEvents_HappyPath(t *testing.T) {
	// Create an instance of Events
	eventsStore := NewEvents()

	// Define test block and transaction
	block := unittest.BlockFixture()
	transaction1 := unittest.TransactionFixture()
	transaction2 := unittest.TransactionFixture()

	event1 := unittest.EventFixture(flow.EventAccountCreated, 0, 0, transaction1.ID(), 200)
	event2 := unittest.EventFixture(flow.EventAccountUpdated, 0, 1, transaction1.ID(), 201)
	event3 := unittest.EventFixture(flow.EventAccountCreated, 1, 2, transaction2.ID(), 202)

	// Store events
	expectedStoredEvents := []flow.EventsList{{event1, event2, event3}}
	err := eventsStore.Store(block.ID(), expectedStoredEvents)
	require.NoError(t, err)

	// Retrieve events by block ID
	storedEvents, err := eventsStore.ByBlockID(block.ID())
	require.NoError(t, err)
	assert.Len(t, storedEvents, len(expectedStoredEvents))
	assert.Contains(t, storedEvents, event1)
	assert.Contains(t, storedEvents, event2)
	assert.Contains(t, storedEvents, event3)

	// Retrieve events by transaction ID
	txEvents, err := eventsStore.ByBlockIDTransactionID(block.ID(), transaction1.ID())
	require.NoError(t, err)
	assert.Len(t, txEvents, 2)
	assert.Equal(t, event1, txEvents[0])
	assert.Equal(t, event2, txEvents[1])

	// Retrieve events by transaction index
	indexEvents, err := eventsStore.ByBlockIDTransactionIndex(block.ID(), 1)
	require.NoError(t, err)
	assert.Len(t, indexEvents, 1)
	assert.Equal(t, event3, indexEvents[0])

	// Retrieve events by event type
	typeEvents, err := eventsStore.ByBlockIDEventType(block.ID(), flow.EventAccountCreated)
	require.NoError(t, err)
	assert.Len(t, typeEvents, 2)
	assert.Contains(t, typeEvents, event1)
	assert.Contains(t, typeEvents, event3)
}
