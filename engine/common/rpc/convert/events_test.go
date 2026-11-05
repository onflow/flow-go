package convert_test

import (
	"encoding/hex"
	"testing"

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
			unittest.Event.WithEventType(flow.EventAccountCreated),
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
			unittest.Event.WithEventType(flow.EventAccountCreated),
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

// TestConvertEventPayloadWithCyclicReference tests converting the CCF payload of an event
// with a cyclic reference value to JSON-CDC.
// Export truncates the cyclic reference value to nil, and both encodings encode it as null.
// See https://github.com/onflow/flow-go/issues/8701
func TestConvertEventPayloadWithCyclicReference(t *testing.T) {
	t.Parallel()

	// Payload of event 4 of mainnet transaction f6dfeb871ff649eb37a5e19e4cfa1c2737da5e63f1a02f5684ba5bd978d57011,
	// emitted by contract RecursiveEventReceiver at 0xe291d9e45266db1e
	ccfPayload, err := hex.DecodeString(
		"d8818282d8a283407830412e653239316439653435323636646231652e5265637572736976654576656e745265636569766572" +
			"2e506f69736f6e818269726563757273697665d8884101d8a08341017833412e653239316439653435323636646231652e5265" +
			"637572736976654576656e7452656365697665722e5265637572736976658182686368696c6472656ed88bd88e82f6d8884101" +
			"82d888408181818181f6",
	)
	require.NoError(t, err)

	jsonPayload, err := convert.CcfPayloadToJsonPayload(ccfPayload)
	require.NoError(t, err)

	require.JSONEq(t,
		`{
		  "type": "Event",
		  "value": {
		    "id": "A.e291d9e45266db1e.RecursiveEventReceiver.Poison",
		    "fields": [
		      {
		        "name": "recursive",
		        "value": {
		          "type": "Struct",
		          "value": {
		            "id": "A.e291d9e45266db1e.RecursiveEventReceiver.Recursive",
		            "fields": [
		              {
		                "name": "children",
		                "value": {
		                  "type": "Array",
		                  "value": [
		                    {
		                      "type": "Struct",
		                      "value": {
		                        "id": "A.e291d9e45266db1e.RecursiveEventReceiver.Recursive",
		                        "fields": [
		                          {
		                            "name": "children",
		                            "value": {
		                              "type": "Array",
		                              "value": [null]
		                            }
		                          }
		                        ]
		                      }
		                    }
		                  ]
		                }
		              }
		            ]
		          }
		        }
		      }
		    ]
		  }
		}`,
		string(jsonPayload),
	)
}

func TestConvertEvents(t *testing.T) {
	t.Parallel()

	eventCount := 3

	events := make([]flow.Event, eventCount)
	ccfEvents := make([]flow.Event, eventCount)
	jsonEvents := make([]flow.Event, eventCount)
	for i := range eventCount {
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
	for i := range count {
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(i)))
		blockEvents[i] = unittest.BlockEventsFixture(header, 2)
	}

	msg, err := convert.BlockEventsToMessages(blockEvents)
	require.NoError(t, err)

	converted, err := convert.MessagesToBlockEvents(msg)
	require.NoError(t, err)

	require.Equal(t, blockEvents, converted)
}
