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
		suite := NewGeneratorSuite(t)
		eventGen := suite.Events()
		identifierGen := suite.Identifiers()

		// Create a single event with custom transaction ID using fixture options
		customTxID := identifierGen.Fixture(t)
		event := eventGen.Fixture(t,
			eventGen.WithTransactionID(customTxID),
			eventGen.WithEventIndex(999),       // Should be reset to 0
			eventGen.WithTransactionIndex(999), // Should be reset to 0
		)

		result := AdjustEventsMetadata([]flow.Event{event})

		require.Len(t, result, 1)
		assert.Equal(t, uint32(0), result[0].EventIndex)
		assert.Equal(t, uint32(0), result[0].TransactionIndex)

		// unchanged
		assert.Equal(t, customTxID, result[0].TransactionID)
		assert.Equal(t, event.Type, result[0].Type)
		assert.Equal(t, event.Payload, result[0].Payload)
	})

	t.Run("multiple events from same transaction", func(t *testing.T) {
		suite := NewGeneratorSuite(t)
		eventGen := suite.Events()
		identifierGen := suite.Identifiers()

		// Create multiple events with same transaction ID but different event indexes using fixture options
		txID := identifierGen.Fixture(t)
		events := []flow.Event{
			eventGen.Fixture(t,
				eventGen.WithTransactionID(txID),
				eventGen.WithTransactionIndex(999), // Should be reset to 0
				eventGen.WithEventIndex(999),       // Should be reset to 0
			),
			eventGen.Fixture(t,
				eventGen.WithTransactionID(txID),
				eventGen.WithTransactionIndex(888), // Should be reset to 0
				eventGen.WithEventIndex(888),       // Should be reset to 1
			),
			eventGen.Fixture(t,
				eventGen.WithTransactionID(txID),
				eventGen.WithTransactionIndex(777), // Should be reset to 0
				eventGen.WithEventIndex(777),       // Should be reset to 2
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
		suite := NewGeneratorSuite(t)
		eventGen := suite.Events()
		identifierGen := suite.Identifiers()

		// Create events from different transactions using fixture options
		// Use the same identifier for events that should be in the same transaction
		txs := []flow.Identifier{
			identifierGen.Fixture(t),
			identifierGen.Fixture(t),
			identifierGen.Fixture(t),
		}

		events := []flow.Event{
			// First transaction - 2 events (both use txID1)
			eventGen.Fixture(t,
				eventGen.WithTransactionID(txs[0]),
				eventGen.WithTransactionIndex(999), // Should be reset to 0
				eventGen.WithEventIndex(999),       // Should be reset to 0
			),
			eventGen.Fixture(t,
				eventGen.WithTransactionID(txs[0]),
				eventGen.WithTransactionIndex(888), // Should be reset to 0
				eventGen.WithEventIndex(888),       // Should be reset to 1
			),
			// Second transaction - 1 event (uses txID2)
			eventGen.Fixture(t,
				eventGen.WithTransactionID(txs[1]),
				eventGen.WithTransactionIndex(777), // Should be reset to 1
				eventGen.WithEventIndex(777),       // Should be reset to 2
			),
			// Third transaction - 3 events (all use txID3)
			eventGen.Fixture(t,
				eventGen.WithTransactionID(txs[2]),
				eventGen.WithTransactionIndex(666), // Should be reset to 2
				eventGen.WithEventIndex(666),       // Should be reset to 3
			),
			eventGen.Fixture(t,
				eventGen.WithTransactionID(txs[2]),
				eventGen.WithTransactionIndex(555), // Should be reset to 2
				eventGen.WithEventIndex(555),       // Should be reset to 4
			),
			eventGen.Fixture(t,
				eventGen.WithTransactionID(txs[2]),
				eventGen.WithTransactionIndex(444), // Should be reset to 2
				eventGen.WithEventIndex(444),       // Should be reset to 5
			),
		}

		result := AdjustEventsMetadata(events)
		require.Len(t, result, 6)

		expectedTxIndexes := []uint32{0, 0, 1, 2, 2, 2}
		for i, event := range result {
			assert.Equal(t, uint32(i), event.EventIndex)

			expectedTxIndex := expectedTxIndexes[i]
			assert.Equal(t, expectedTxIndex, event.TransactionIndex)
			assert.Equal(t, txs[expectedTxIndex], event.TransactionID)
		}
	})
}
