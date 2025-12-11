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

	cadenceValue := cadence.NewInt(2)

	t.Run("convert json cdc encoded event", func(t *testing.T) {
		ccfPayload, err := ccf.Encode(cadenceValue)
		require.NoError(t, err)

		event := unittest.EventFixture(
			unittest.Event.WithPayload(ccfPayload),
		)

		msg := convert.EventToMessage(event)
		converted, err := convert.MessageToEvent(msg)
		require.NoError(t, err)

		require.Equal(t, event, *converted)
	})

	t.Run("convert json cdc encoded event", func(t *testing.T) {
		jsonPayload, err := jsoncdc.Encode(cadenceValue)
		require.NoError(t, err)

		event := unittest.EventFixture(
			unittest.Event.WithPayload(jsonPayload),
		)

		msg := convert.EventToMessage(event)
		converted, err := convert.MessageToEvent(msg)
		require.NoError(t, err)

		require.Equal(t, event.Type, converted.Type)
	})
}

// TestConvertEventWithPayloadConversion tests converting events to and from protobuf messages
// with payload modification
func TestConvertEventWithPayloadConversion(t *testing.T) {
	t.Parallel()

	cadenceValue := cadence.NewInt(2)

	payload, err := ccf.Encode(cadenceValue)
	require.NoError(t, err)

	ccfEvent := unittest.EventFixture(
		unittest.Event.WithEventType(flow.EventAccountCreated),
		unittest.Event.WithPayload(payload),
	)

	payload, err = jsoncdc.Encode(cadenceValue)
	require.NoError(t, err)

	jsonEvent := unittest.EventFixture(
		unittest.Event.WithEventType(flow.EventAccountCreated),
		unittest.Event.WithPayload(payload),
	)

	t.Run("convert payload from ccf to jsoncdc", func(t *testing.T) {
		message := convert.EventToMessage(ccfEvent)
		convertedEvent, err := convert.MessageToEventFromVersion(message, entities.EventEncodingVersion_CCF_V0)
		require.NoError(t, err)

		require.Equal(t, jsonEvent, *convertedEvent)
	})

	t.Run("convert payload from jsoncdc to jsoncdc", func(t *testing.T) {
		message := convert.EventToMessage(jsonEvent)
		convertedEvent, err := convert.MessageToEventFromVersion(message, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)

		require.Equal(t, jsonEvent, *convertedEvent)
	})
}

func TestConvertEvents(t *testing.T) {
	t.Parallel()

	eventCount := 3

	events := make([]flow.Event, eventCount)
	ccfEvents := make([]flow.Event, eventCount)
	jsonEvents := make([]flow.Event, eventCount)
	for i := 0; i < eventCount; i++ {
		cadenceValue := cadence.NewInt(i)

		ccfPayload, err := ccf.Encode(cadenceValue)
		require.NoError(t, err)

		jsonPayload, err := jsoncdc.Encode(cadenceValue)
		require.NoError(t, err)

		event := unittest.EventFixture()
		ccfEvent := unittest.EventFixture(
			unittest.Event.WithEventType(flow.EventAccountCreated),
			unittest.Event.WithPayload(ccfPayload),
		)
		jsonEvent := unittest.EventFixture(
			unittest.Event.WithEventType(flow.EventAccountCreated),
			unittest.Event.WithPayload(jsonPayload),
		)

		events[i] = event
		ccfEvents[i] = ccfEvent
		jsonEvents[i] = jsonEvent
	}

	t.Run("empty", func(t *testing.T) {
		messages := convert.EventsToMessages(nil)
		require.Len(t, messages, 0)
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

		converted, err := convert.MessagesToEvents(messages)
		require.NoError(t, err)

		require.Equal(t, events, converted)
	})

	t.Run("convert event from ccf to jsoncdc", func(t *testing.T) {
		messages := convert.EventsToMessages(ccfEvents)
		converted, err := convert.MessagesToEventsWithEncodingConversion(messages, entities.EventEncodingVersion_CCF_V0, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)

		require.Equal(t, jsonEvents, converted)
	})

	t.Run("convert event from jsoncdc", func(t *testing.T) {
		messages := convert.EventsToMessages(jsonEvents)
		converted, err := convert.MessagesToEventsWithEncodingConversion(messages, entities.EventEncodingVersion_JSON_CDC_V0, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)

		require.Equal(t, jsonEvents, converted)
	})

	t.Run("convert event from ccf", func(t *testing.T) {
		messages := convert.EventsToMessages(jsonEvents)
		converted, err := convert.MessagesToEventsWithEncodingConversion(messages, entities.EventEncodingVersion_CCF_V0, entities.EventEncodingVersion_CCF_V0)
		require.NoError(t, err)

		require.Equal(t, jsonEvents, converted)
	})

	t.Run("convert event from jsoncdc to ccf", func(t *testing.T) {
		messages := convert.EventsToMessages(jsonEvents)
		converted, err := convert.MessagesToEventsWithEncodingConversion(messages, entities.EventEncodingVersion_JSON_CDC_V0, entities.EventEncodingVersion_CCF_V0)
		require.Error(t, err)
		require.Nil(t, converted)
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

	require.Equal(t, serviceEvents[0], *converted)
}

func TestConvertServiceEventList(t *testing.T) {
	t.Parallel()

	serviceEvents := unittest.ServiceEventsFixture(5)
	require.Len(t, serviceEvents, 5)

	msg, err := convert.ServiceEventListToMessages(serviceEvents)
	require.NoError(t, err)

	converted, err := convert.MessagesToServiceEventList(msg)
	require.NoError(t, err)

	require.Equal(t, serviceEvents, converted)
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

	converted, err := convert.MessagesToBlockEvents(msg)
	require.NoError(t, err)

	require.Equal(t, blockEvents, converted)
}

// TestConvertBlockEvent tests round-trip converting a single protobuf message.
func TestConvertBlockEvent(t *testing.T) {
	t.Parallel()

	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(42))
	blockEvents := unittest.BlockEventsFixture(header, 3)

	// Convert to message first
	msg, err := convert.BlockEventsToMessage(blockEvents)
	require.NoError(t, err)

	// Convert back to BlockEvents
	converted, err := convert.MessageToBlockEvents(msg)
	require.NoError(t, err)
	require.NotNil(t, converted)

	require.Equal(t, blockEvents.BlockHeight, converted.BlockHeight)
	require.Equal(t, blockEvents.BlockID, converted.BlockID)
	require.Equal(t, blockEvents.BlockTimestamp, converted.BlockTimestamp)
	require.Len(t, converted.Events, len(blockEvents.Events))
	for i, event := range blockEvents.Events {
		require.Equal(t, event.ID(), converted.Events[i].ID())
	}
}

// TestConvertCcfEventToJsonEvent tests converting a single CCF event to JSON event.
func TestConvertCcfEventToJsonEvent(t *testing.T) {
	t.Parallel()

	t.Run("converts CCF event to JSON event", func(t *testing.T) {
		t.Parallel()

		// Prepare input CCF event and expected JSON output
		cadenceValue := cadence.NewInt(42)
		ccfPayload, err := ccf.Encode(cadenceValue)
		require.NoError(t, err)

		expectedJsonPayload, err := jsoncdc.Encode(cadenceValue)
		require.NoError(t, err)

		ccfEvent := unittest.EventFixture(
			unittest.Event.WithPayload(ccfPayload),
		)

		jsonEvent, err := convert.CcfEventToJsonEvent(ccfEvent)
		require.NoError(t, err)
		require.NotNil(t, jsonEvent)

		require.Equal(t, ccfEvent.Type, jsonEvent.Type)
		require.Equal(t, ccfEvent.TransactionID, jsonEvent.TransactionID)
		require.Equal(t, ccfEvent.TransactionIndex, jsonEvent.TransactionIndex)
		require.Equal(t, ccfEvent.EventIndex, jsonEvent.EventIndex)

		// Verify payload matches expected JSON-CDC encoding
		require.Equal(t, expectedJsonPayload, jsonEvent.Payload)
	})

	t.Run("returns error on invalid CCF payload", func(t *testing.T) {
		t.Parallel()

		invalidEvent := unittest.EventFixture(
			unittest.Event.WithPayload([]byte{0x01, 0x02, 0x03}), // Invalid CCF
		)

		jsonEvent, err := convert.CcfEventToJsonEvent(invalidEvent)
		require.Nil(t, jsonEvent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to decode from ccf format")
	})

	t.Run("returns error on empty payload", func(t *testing.T) {
		t.Parallel()

		emptyEvent := unittest.EventFixture(
			unittest.Event.WithPayload([]byte{}),
		)

		jsonEvent, err := convert.CcfEventToJsonEvent(emptyEvent)
		require.Nil(t, jsonEvent)
		require.Error(t, err, "empty payload should result in error from flow.NewEvent")
		assert.Contains(t, err.Error(), "payload must not be empty")
	})
}

// TestConvertCcfPayloadToJsonPayload tests converting CCF-encoded payloads to JSON-encoded payloads.
func TestConvertCcfPayloadToJsonPayload(t *testing.T) {
	t.Parallel()

	t.Run("convert ccf payload to json payload", func(t *testing.T) {
		t.Parallel()

		cadenceValue := cadence.NewInt(42)
		ccfPayload, err := ccf.Encode(cadenceValue)
		require.NoError(t, err)

		jsonPayload, err := convert.CcfPayloadToJsonPayload(ccfPayload)
		require.NoError(t, err)

		decoded, err := jsoncdc.Decode(nil, jsonPayload)
		require.NoError(t, err)
		require.Equal(t, cadenceValue, decoded)
	})

	t.Run("empty payload returns empty", func(t *testing.T) {
		t.Parallel()

		result, err := convert.CcfPayloadToJsonPayload([]byte{})
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("invalid ccf payload returns error", func(t *testing.T) {
		t.Parallel()

		invalidPayload := []byte{0x01, 0x02, 0x03}
		jsonPayload, err := convert.CcfPayloadToJsonPayload(invalidPayload)
		require.Nil(t, jsonPayload)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to decode from ccf format")
	})
}

// TestConvertCcfEventsToJsonEvents tests converting multiple CCF events to JSON events.
func TestConvertCcfEventsToJsonEvents(t *testing.T) {
	t.Parallel()

	t.Run("converts multiple events correctly", func(t *testing.T) {
		t.Parallel()

		eventCount := 5
		ccfEvents := make([]flow.Event, eventCount)

		// Create test events with varying payloads
		for i := 0; i < eventCount; i++ {
			cadenceValue := cadence.NewInt(i)
			ccfPayload, err := ccf.Encode(cadenceValue)
			require.NoError(t, err)

			ccfEvents[i] = unittest.EventFixture(
				unittest.Event.WithPayload(ccfPayload),
			)
		}

		converted, err := convert.CcfEventsToJsonEvents(ccfEvents)
		require.NoError(t, err)
		require.Len(t, converted, len(ccfEvents))

		for i, convertedEvent := range converted {
			require.Equal(t, ccfEvents[i].Type, convertedEvent.Type)
			require.Equal(t, ccfEvents[i].TransactionID, convertedEvent.TransactionID)
			require.Equal(t, ccfEvents[i].TransactionIndex, convertedEvent.TransactionIndex)
			require.Equal(t, ccfEvents[i].EventIndex, convertedEvent.EventIndex)

			decoded, err := jsoncdc.Decode(nil, convertedEvent.Payload)
			require.NoError(t, err, "payload should be valid JSON-CDC")
			require.Equal(t, cadence.NewInt(i), decoded, "decoded value should match original")
		}
	})

	t.Run("returns error on invalid CCF payload", func(t *testing.T) {
		t.Parallel()

		invalidEvents := []flow.Event{
			unittest.EventFixture(
				unittest.Event.WithPayload([]byte{0x01, 0x02, 0x03}), // Invalid CCF
			),
		}

		jsonEvents, err := convert.CcfEventsToJsonEvents(invalidEvents)
		require.Nil(t, jsonEvents)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to decode from ccf format")
	})
}

// TestConvertEventToMessageFromVersion tests converting events to messages with version-specific encoding.
func TestConvertEventToMessageFromVersion(t *testing.T) {
	t.Parallel()

	cadenceValue := cadence.NewInt(123)

	t.Run("convert ccf event to json message", func(t *testing.T) {
		t.Parallel()

		ccfPayload, err := ccf.Encode(cadenceValue)
		require.NoError(t, err)

		event := unittest.EventFixture(
			unittest.Event.WithPayload(ccfPayload),
		)

		message, err := convert.EventToMessageFromVersion(event, entities.EventEncodingVersion_CCF_V0)
		require.NoError(t, err)

		decoded, err := jsoncdc.Decode(nil, message.Payload)
		require.NoError(t, err)
		require.Equal(t, cadenceValue, decoded)
	})

	t.Run("convert json event to json message", func(t *testing.T) {
		t.Parallel()

		jsonPayload, err := jsoncdc.Encode(cadenceValue)
		require.NoError(t, err)

		event := unittest.EventFixture(
			unittest.Event.WithPayload(jsonPayload),
		)

		message, err := convert.EventToMessageFromVersion(event, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)

		require.Equal(t, jsonPayload, message.Payload)
	})

	t.Run("empty payload is handled correctly", func(t *testing.T) {
		t.Parallel()

		event := unittest.EventFixture(
			unittest.Event.WithEventType(flow.EventAccountCreated),
			unittest.Event.WithPayload([]byte{}),
		)

		message, err := convert.EventToMessageFromVersion(event, entities.EventEncodingVersion_CCF_V0)
		require.NoError(t, err)
		require.Empty(t, message.Payload)
	})

	t.Run("invalid version returns error", func(t *testing.T) {
		t.Parallel()

		event := unittest.EventFixture()

		message, err := convert.EventToMessageFromVersion(event, entities.EventEncodingVersion(999))
		require.Nil(t, message)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid encoding format")
	})
}

// TestConvertEventsToMessagesWithEncodingConversion tests batch conversion with encoding changes.
func TestConvertEventsToMessagesWithEncodingConversion(t *testing.T) {
	t.Parallel()

	eventCount := 3
	ccfEvents := make([]flow.Event, eventCount)

	for i := 0; i < eventCount; i++ {
		cadenceValue := cadence.NewInt(i)
		ccfPayload, err := ccf.Encode(cadenceValue)
		require.NoError(t, err)

		ccfEvents[i] = unittest.EventFixture(
			unittest.Event.WithEventType(flow.EventAccountCreated),
			unittest.Event.WithPayload(ccfPayload),
		)
	}

	t.Run("convert from ccf to json", func(t *testing.T) {
		t.Parallel()

		messages, err := convert.EventsToMessagesWithEncodingConversion(
			ccfEvents,
			entities.EventEncodingVersion_CCF_V0,
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		require.NoError(t, err)
		require.Len(t, messages, len(ccfEvents))

		for i, msg := range messages {
			decoded, err := jsoncdc.Decode(nil, msg.Payload)
			require.NoError(t, err)
			require.Equal(t, cadence.NewInt(i), decoded)
		}
	})

	t.Run("same version uses passthrough", func(t *testing.T) {
		t.Parallel()

		messages, err := convert.EventsToMessagesWithEncodingConversion(
			ccfEvents,
			entities.EventEncodingVersion_CCF_V0,
			entities.EventEncodingVersion_CCF_V0,
		)
		require.NoError(t, err)
		require.Len(t, messages, len(ccfEvents))
	})

	t.Run("json to ccf conversion is not supported", func(t *testing.T) {
		t.Parallel()

		jsonEvents := make([]flow.Event, 1)
		jsonPayload, err := jsoncdc.Encode(cadence.NewInt(1))
		require.NoError(t, err)

		jsonEvents[0] = unittest.EventFixture(
			unittest.Event.WithPayload(jsonPayload),
		)

		message, err := convert.EventsToMessagesWithEncodingConversion(
			jsonEvents,
			entities.EventEncodingVersion_JSON_CDC_V0,
			entities.EventEncodingVersion_CCF_V0,
		)
		require.Nil(t, message)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conversion from format")
	})
}
