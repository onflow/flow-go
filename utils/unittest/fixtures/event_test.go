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
		identifierGen := suite.Identifiers()

		txID := identifierGen.Fixture()
		event := eventGen.Fixture(
			eventGen.WithTransactionID(txID),
			eventGen.WithEventIndex(999),
			eventGen.WithTransactionIndex(999),
		)

		result := AdjustEventsMetadata([]flow.Event{event})
		require.Len(t, result, 1)

		assert.Equal(t, uint32(0), result[0].EventIndex)
		assert.Equal(t, uint32(0), result[0].TransactionIndex)

		// unchanged
		assert.Equal(t, txID, result[0].TransactionID)
		assert.Equal(t, event.Type, result[0].Type)
		assert.Equal(t, event.Payload, result[0].Payload)
	})

	t.Run("multiple events from same transaction", func(t *testing.T) {
		suite := NewGeneratorSuite()
		eventGen := suite.Events()
		identifierGen := suite.Identifiers()

		txID := identifierGen.Fixture()
		events := []flow.Event{
			eventGen.Fixture(
				eventGen.WithTransactionID(txID),
				eventGen.WithTransactionIndex(999),
				eventGen.WithEventIndex(999),
			),
			eventGen.Fixture(
				eventGen.WithTransactionID(txID),
				eventGen.WithTransactionIndex(888),
				eventGen.WithEventIndex(888),
			),
			eventGen.Fixture(
				eventGen.WithTransactionID(txID),
				eventGen.WithTransactionIndex(777),
				eventGen.WithEventIndex(777),
			),
		}

		result := AdjustEventsMetadata(events)
		require.Len(t, result, 3)

		for i, event := range result {
			assert.Equal(t, txID, event.TransactionID)
			assert.Equal(t, uint32(0), event.TransactionIndex)
			assert.Equal(t, uint32(i), event.EventIndex)
		}
	})

	t.Run("multiple events from different transactions", func(t *testing.T) {
		suite := NewGeneratorSuite()
		eventGen := suite.Events()
		identifierGen := suite.Identifiers()
		randomGen := suite.Random()

		txID0 := identifierGen.Fixture()
		txID1 := identifierGen.Fixture()
		txID2 := identifierGen.Fixture()

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
				eventGen.WithTransactionID(event.transactionID),
				eventGen.WithTransactionIndex(randomGen.Uint32()),
				eventGen.WithEventIndex(randomGen.Uint32()),
			)
		}

		result := AdjustEventsMetadata(events)
		require.Len(t, result, len(expected))

		for i, event := range result {
			assert.Equal(t, expected[i].transactionID, event.TransactionID)
			assert.Equal(t, expected[i].transactionIndex, event.TransactionIndex)
			assert.Equal(t, expected[i].eventIndex, event.EventIndex)
		}
	})
}
