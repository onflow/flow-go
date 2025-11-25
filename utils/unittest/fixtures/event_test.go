package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func TestAdjustEventsMetadata(t *testing.T) {
	t.Parallel()

	t.Run("empty events list", func(t *testing.T) {
		result := AdjustEventsMetadata([]flow.Event{})
		assert.Empty(t, result)
	})

	t.Run("single event", func(t *testing.T) {
		suite := NewGeneratorSuite()
		eventGen := suite.Events()
		identifier := suite.Identifiers()

		txID := identifier.Fixture()
		event := eventGen.Fixture(
			Event.WithTransactionID(txID),
			Event.WithEventIndex(999),
			Event.WithTransactionIndex(999),
		)

		result := AdjustEventsMetadata([]flow.Event{event})
		require.Len(t, result, 1)

		assert.Equal(t, uint32(0), result[0].EventIndex)
		assert.Equal(t, uint32(999), result[0].TransactionIndex)

		// unchanged
		assert.Equal(t, txID, result[0].TransactionID)
		assert.Equal(t, event.Type, result[0].Type)
		assert.Equal(t, event.Payload, result[0].Payload)
	})

	t.Run("multiple events from same transaction", func(t *testing.T) {
		suite := NewGeneratorSuite()
		eventGen := suite.Events()
		identifier := suite.Identifiers()

		txID := identifier.Fixture()
		events := []flow.Event{
			eventGen.Fixture(
				Event.WithTransactionID(txID),
				Event.WithTransactionIndex(999),
				Event.WithEventIndex(999),
			),
			eventGen.Fixture(
				Event.WithTransactionID(txID),
				Event.WithTransactionIndex(888),
				Event.WithEventIndex(888),
			),
			eventGen.Fixture(
				Event.WithTransactionID(txID),
				Event.WithTransactionIndex(777),
				Event.WithEventIndex(777),
			),
		}

		result := AdjustEventsMetadata(events)
		require.Len(t, result, 3)

		for i, event := range result {
			assert.Equal(t, txID, event.TransactionID)
			// all tx have the same txID, so they should all have the same txIndex
			assert.Equal(t, uint32(999), event.TransactionIndex)
			assert.Equal(t, uint32(i), event.EventIndex)
		}
	})

	t.Run("multiple events from different transactions", func(t *testing.T) {
		suite := NewGeneratorSuite()
		eventGen := suite.Events()
		identifier := suite.Identifiers()
		random := suite.Random()

		txID0 := identifier.Fixture()
		txID1 := identifier.Fixture()
		txID2 := identifier.Fixture()

		type eventConfig struct {
			transactionID    flow.Identifier
			transactionIndex uint32
			eventIndex       uint32
		}

		expected := []eventConfig{
			{
				transactionID:    txID0,
				transactionIndex: 0,
				eventIndex:       0,
			},
			{
				transactionID:    txID0,
				transactionIndex: 0,
				eventIndex:       1,
			},
			{
				transactionID:    txID1,
				transactionIndex: 1,
				eventIndex:       0,
			},
			{
				transactionID:    txID2,
				transactionIndex: 2,
				eventIndex:       0,
			},
			{
				transactionID:    txID2,
				transactionIndex: 2,
				eventIndex:       1,
			},
			{
				transactionID:    txID2,
				transactionIndex: 2,
				eventIndex:       2,
			},
		}

		events := make([]flow.Event, len(expected))
		for i, event := range expected {
			events[i] = eventGen.Fixture(
				Event.WithTransactionID(event.transactionID),
				Event.WithTransactionIndex(random.Uint32()),
				Event.WithEventIndex(random.Uint32()),
			)
		}

		// will adjust according to the first tx's index
		events[0].TransactionIndex = 0

		result := AdjustEventsMetadata(events)
		require.Len(t, result, len(expected))

		for i, event := range result {
			assert.Equal(t, expected[i].transactionID, event.TransactionID)
			assert.Equal(t, expected[i].transactionIndex, event.TransactionIndex)
			assert.Equal(t, expected[i].eventIndex, event.EventIndex)
		}
	})
}
