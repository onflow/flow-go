package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConvertEventWithoutPayloadConversion tests converting events to and from protobuf messages
// with no payload modification
func TestConvertEventWithoutPayloadConversion(t *testing.T) {
	t.Parallel()

	txID := unittest.IdentifierFixture()
	cadenceValue, err := cadence.NewValue(2)
	require.NoError(t, err)

	t.Run("convert empty event", func(t *testing.T) {
		event := unittest.EventFixture(flow.EventAccountCreated, 2, 3, txID, 0)

		msg := convert.EventToMessage(event)
		converted := convert.MessageToEvent(msg)

		assert.Equal(t, event, converted)
	})

	t.Run("convert json cdc encoded event", func(t *testing.T) {
		ccfPayload, err := ccf.Encode(cadenceValue)
		require.NoError(t, err)

		event := unittest.EventFixture(flow.EventAccountCreated, 2, 3, txID, 0)
		event.Payload = ccfPayload

		msg := convert.EventToMessage(event)
		converted := convert.MessageToEvent(msg)

		assert.Equal(t, event, converted)
	})

	t.Run("convert json cdc encoded event", func(t *testing.T) {
		jsonPayload, err := jsoncdc.Encode(cadenceValue)
		require.NoError(t, err)

		event := unittest.EventFixture(flow.EventAccountCreated, 2, 3, txID, 0)
		event.Payload = jsonPayload

		msg := convert.EventToMessage(event)
		converted := convert.MessageToEvent(msg)

		assert.Equal(t, event.Type, converted.Type)
	})
}

// TestConvertEventWithPayloadConversion tests converting events to and from protobuf messages
// with payload modification
func TestConvertEventWithPayloadConversion(t *testing.T) {
	t.Parallel()

	txID := unittest.IdentifierFixture()
	cadenceValue, err := cadence.NewValue(2)
	require.NoError(t, err)

	ccfEvent := unittest.EventFixture(flow.EventAccountCreated, 2, 3, txID, 0)
	ccfEvent.Payload, err = ccf.Encode(cadenceValue)
	require.NoError(t, err)

	jsonEvent := unittest.EventFixture(flow.EventAccountCreated, 2, 3, txID, 0)
	jsonEvent.Payload, err = jsoncdc.Encode(cadenceValue)
	require.NoError(t, err)

	t.Run("convert payload from ccf to jsoncdc", func(t *testing.T) {
		message := convert.EventToMessage(ccfEvent)
		convertedEvent, err := convert.MessageToEventFromVersion(message, entities.EventEncodingVersion_CCF_V0)
		assert.NoError(t, err)

		assert.Equal(t, jsonEvent, *convertedEvent)
	})

	t.Run("convert payload from jsoncdc to jsoncdc", func(t *testing.T) {
		message := convert.EventToMessage(jsonEvent)
		convertedEvent, err := convert.MessageToEventFromVersion(message, entities.EventEncodingVersion_JSON_CDC_V0)
		assert.NoError(t, err)

		assert.Equal(t, jsonEvent, *convertedEvent)
	})
}

func TestConvertEvents(t *testing.T) {
	t.Parallel()

	eventCount := 3
	txID := unittest.IdentifierFixture()

	events := make([]flow.Event, eventCount)
	ccfEvents := make([]flow.Event, eventCount)
	jsonEvents := make([]flow.Event, eventCount)
	for i := 0; i < eventCount; i++ {
		cadenceValue, err := cadence.NewValue(i)
		require.NoError(t, err)

		eventIndex := 3 + uint32(i)

		event := unittest.EventFixture(flow.EventAccountCreated, 2, eventIndex, txID, 0)
		ccfEvent := unittest.EventFixture(flow.EventAccountCreated, 2, eventIndex, txID, 0)
		jsonEvent := unittest.EventFixture(flow.EventAccountCreated, 2, eventIndex, txID, 0)

		ccfEvent.Payload, err = ccf.Encode(cadenceValue)
		require.NoError(t, err)

		jsonEvent.Payload, err = jsoncdc.Encode(cadenceValue)
		require.NoError(t, err)

		events[i] = event
		ccfEvents[i] = ccfEvent
		jsonEvents[i] = jsonEvent
	}

	t.Run("empty", func(t *testing.T) {
		messages := convert.EventsToMessages(nil)
		assert.Len(t, messages, 0)
	})

	t.Run("convert with passthrough payload conversion", func(t *testing.T) {
		messages := convert.EventsToMessages(events)
		require.Len(t, messages, len(events))

		for i, message := range messages {
			event := events[i]
			require.Equal(t, event.EventIndex, message.EventIndex)
			require.Equal(t, event.TransactionIndex, message.TransactionIndex)
			require.Equal(t, event.Payload, message.Payload)
			require.Equal(t, event.TransactionID[:], message.TransactionId)
			require.Equal(t, string(event.Type), message.Type)
		}

		converted := convert.MessagesToEvents(messages)
		assert.Equal(t, events, converted)
	})

	t.Run("convert event from ccf to jsoncdc", func(t *testing.T) {
		messages := convert.EventsToMessages(ccfEvents)
		converted, err := convert.MessagesToEventsWithEncodingConversion(messages, entities.EventEncodingVersion_CCF_V0, entities.EventEncodingVersion_JSON_CDC_V0)
		assert.NoError(t, err)

		assert.Equal(t, jsonEvents, converted)
	})

	t.Run("convert event from jsoncdc", func(t *testing.T) {
		messages := convert.EventsToMessages(jsonEvents)
		converted, err := convert.MessagesToEventsWithEncodingConversion(messages, entities.EventEncodingVersion_JSON_CDC_V0, entities.EventEncodingVersion_JSON_CDC_V0)
		assert.NoError(t, err)

		assert.Equal(t, jsonEvents, converted)
	})

	t.Run("convert event from ccf", func(t *testing.T) {
		messages := convert.EventsToMessages(jsonEvents)
		converted, err := convert.MessagesToEventsWithEncodingConversion(messages, entities.EventEncodingVersion_CCF_V0, entities.EventEncodingVersion_CCF_V0)
		assert.NoError(t, err)

		assert.Equal(t, jsonEvents, converted)
	})

	t.Run("convert event from jsoncdc to ccf", func(t *testing.T) {
		messages := convert.EventsToMessages(jsonEvents)
		converted, err := convert.MessagesToEventsWithEncodingConversion(messages, entities.EventEncodingVersion_JSON_CDC_V0, entities.EventEncodingVersion_CCF_V0)
		assert.Error(t, err)
		assert.Nil(t, converted)
	})
}

func TestConvertServiceEvent(t *testing.T) {
	t.Parallel()

	serviceEvents := unittest.ServiceEventsFixture(1)
	require.Len(t, serviceEvents, 1)

	msg, err := convert.ServiceEventToMessage(serviceEvents[0])
	require.NoError(t, err)

	converted, err := convert.MessageToServiceEvent(msg)
	require.NoError(t, err)

	assert.Equal(t, serviceEvents[0], *converted)
}

func TestConvertServiceEventList(t *testing.T) {
	t.Parallel()

	serviceEvents := unittest.ServiceEventsFixture(5)
	require.Len(t, serviceEvents, 5)

	msg, err := convert.ServiceEventListToMessages(serviceEvents)
	require.NoError(t, err)

	converted, err := convert.MessagesToServiceEventList(msg)
	require.NoError(t, err)

	assert.Equal(t, serviceEvents, converted)
}

// TestConvertMessagesToBlockEvents tests that converting a protobuf EventsResponse_Result message to and from block events in the same
// block
func TestConvertMessagesToBlockEvents(t *testing.T) {
	t.Parallel()

	count := 2
	blockEvents := make([]flow.BlockEvents, count)
	for i := 0; i < count; i++ {
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(i)))
		blockEvents[i] = unittest.BlockEventsFixture(header, 2)
	}

	msg, err := convert.BlockEventsToMessages(blockEvents)
	require.NoError(t, err)

	converted := convert.MessagesToBlockEvents(msg)
	require.NoError(t, err)

	assert.Equal(t, blockEvents, converted)
}
