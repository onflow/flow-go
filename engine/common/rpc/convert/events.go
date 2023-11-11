package convert

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/model/flow"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// EventToMessage converts a flow.Event to a protobuf message
// Note: this function does not convert the payload encoding
func EventToMessage(e flow.Event) *entities.Event {
	return &entities.Event{
		Type:             string(e.Type),
		TransactionId:    e.TransactionID[:],
		TransactionIndex: e.TransactionIndex,
		EventIndex:       e.EventIndex,
		Payload:          e.Payload,
	}
}

// MessageToEvent converts a protobuf message to a flow.Event
// Note: this function does not convert the payload encoding
func MessageToEvent(m *entities.Event) flow.Event {
	return flow.Event{
		Type:             flow.EventType(m.GetType()),
		TransactionID:    flow.HashToID(m.GetTransactionId()),
		TransactionIndex: m.GetTransactionIndex(),
		EventIndex:       m.GetEventIndex(),
		Payload:          m.GetPayload(),
	}
}

// EventsToMessages converts a slice of flow.Events to a slice of protobuf messages
// Note: this function does not convert the payload encoding
func EventsToMessages(flowEvents []flow.Event) []*entities.Event {
	events := make([]*entities.Event, len(flowEvents))
	for i, e := range flowEvents {
		event := EventToMessage(e)
		events[i] = event
	}
	return events
}

// MessagesToEvents converts a slice of protobuf messages to a slice of flow.Events
// Note: this function does not convert the payload encoding
func MessagesToEvents(l []*entities.Event) []flow.Event {
	events := make([]flow.Event, len(l))
	for i, m := range l {
		events[i] = MessageToEvent(m)
	}
	return events
}

// EventToMessageFromVersion converts a flow.Event to a protobuf message, converting the payload
// encoding from CCF to JSON if the input version is CCF
func EventToMessageFromVersion(e flow.Event, version entities.EventEncodingVersion) (*entities.Event, error) {
	message := EventToMessage(e)

	if len(e.Payload) > 0 {
		switch version {
		case entities.EventEncodingVersion_CCF_V0:
			convertedPayload, err := CcfPayloadToJsonPayload(e.Payload)
			if err != nil {
				return nil, fmt.Errorf("could not convert event payload from CCF to Json: %w", err)
			}
			message.Payload = convertedPayload
		case entities.EventEncodingVersion_JSON_CDC_V0:
		default:
			return nil, fmt.Errorf("invalid encoding format %d", version)
		}
	}

	return message, nil
}

// MessageToEventFromVersion converts a protobuf message to a flow.Event, and converts the payload
// encoding from CCF to JSON if the input version is CCF
func MessageToEventFromVersion(m *entities.Event, inputVersion entities.EventEncodingVersion) (*flow.Event, error) {
	event := MessageToEvent(m)
	switch inputVersion {
	case entities.EventEncodingVersion_CCF_V0:
		convertedPayload, err := CcfPayloadToJsonPayload(event.Payload)
		if err != nil {
			return nil, fmt.Errorf("could not convert event payload from CCF to Json: %w", err)
		}
		event.Payload = convertedPayload
		return &event, nil
	case entities.EventEncodingVersion_JSON_CDC_V0:
		return &event, nil
	default:
		return nil, fmt.Errorf("invalid encoding format %d", inputVersion)
	}
}

// EventsToMessagesWithEncodingConversion converts a slice of flow.Events to a slice of protobuf messages, converting
// the payload encoding from CCF to JSON if the input version is CCF
func EventsToMessagesWithEncodingConversion(
	flowEvents []flow.Event,
	from entities.EventEncodingVersion,
	to entities.EventEncodingVersion,
) ([]*entities.Event, error) {
	if from == entities.EventEncodingVersion_JSON_CDC_V0 && to == entities.EventEncodingVersion_CCF_V0 {
		return nil, fmt.Errorf("conversion from format %s to %s is not supported", from.String(), to.String())
	}

	if from == to {
		return EventsToMessages(flowEvents), nil
	}

	events := make([]*entities.Event, len(flowEvents))
	for i, e := range flowEvents {
		event, err := EventToMessageFromVersion(e, from)
		if err != nil {
			return nil, fmt.Errorf("could not convert event at index %d from format %d: %w",
				e.EventIndex, from, err)
		}
		events[i] = event
	}
	return events, nil
}

// MessagesToEventsWithEncodingConversion converts a slice of protobuf messages to a slice of flow.Events, converting
// the payload encoding from CCF to JSON if the input version is CCF
func MessagesToEventsWithEncodingConversion(
	messageEvents []*entities.Event,
	from entities.EventEncodingVersion,
	to entities.EventEncodingVersion,
) ([]flow.Event, error) {
	if from == entities.EventEncodingVersion_JSON_CDC_V0 && to == entities.EventEncodingVersion_CCF_V0 {
		return nil, fmt.Errorf("conversion from format %s to %s is not supported", from.String(), to.String())
	}

	if from == to {
		return MessagesToEvents(messageEvents), nil
	}

	events := make([]flow.Event, len(messageEvents))
	for i, m := range messageEvents {
		event, err := MessageToEventFromVersion(m, from)
		if err != nil {
			return nil, fmt.Errorf("could not convert event at index %d from format %d: %w",
				m.EventIndex, from, err)
		}
		events[i] = *event
	}
	return events, nil
}

// ServiceEventToMessage converts a flow.ServiceEvent to a protobuf message
func ServiceEventToMessage(event flow.ServiceEvent) (*entities.ServiceEvent, error) {
	bytes, err := json.Marshal(event.Event)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal service event: %w", err)
	}

	return &entities.ServiceEvent{
		Type:    event.Type.String(),
		Payload: bytes,
	}, nil
}

// MessageToServiceEvent converts a protobuf message to a flow.ServiceEvent
func MessageToServiceEvent(m *entities.ServiceEvent) (*flow.ServiceEvent, error) {
	rawEvent := m.Payload
	eventType := flow.ServiceEventType(m.Type)
	se, err := flow.ServiceEventJSONMarshaller.UnmarshalWithType(rawEvent, eventType)

	return &se, err
}

// ServiceEventListToMessages converts a slice of flow.ServiceEvents to a slice of protobuf messages
func ServiceEventListToMessages(list flow.ServiceEventList) (
	[]*entities.ServiceEvent,
	error,
) {
	entities := make([]*entities.ServiceEvent, len(list))
	for i, serviceEvent := range list {
		m, err := ServiceEventToMessage(serviceEvent)
		if err != nil {
			return nil, fmt.Errorf("failed to convert service event at index %d to message: %w", i, err)
		}
		entities[i] = m
	}
	return entities, nil
}

// MessagesToServiceEventList converts a slice of flow.ServiceEvents to a slice of protobuf messages
func MessagesToServiceEventList(m []*entities.ServiceEvent) (
	flow.ServiceEventList,
	error,
) {
	parsedServiceEvents := make(flow.ServiceEventList, len(m))
	for i, serviceEvent := range m {
		parsedServiceEvent, err := MessageToServiceEvent(serviceEvent)
		if err != nil {
			return nil, fmt.Errorf("failed to parse service event at index %d from message: %w", i, err)
		}
		parsedServiceEvents[i] = *parsedServiceEvent
	}
	return parsedServiceEvents, nil
}

// CcfPayloadToJsonPayload converts a CCF-encoded payload to a JSON-encoded payload
func CcfPayloadToJsonPayload(p []byte) ([]byte, error) {
	if len(p) == 0 {
		return p, nil
	}

	val, err := ccf.Decode(nil, p)
	if err != nil {
		return nil, fmt.Errorf("unable to decode from ccf format: %w", err)
	}
	res, err := jsoncdc.Encode(val)
	if err != nil {
		return nil, fmt.Errorf("unable to encode to json-cdc format: %w", err)
	}
	return res, nil
}

// CcfEventToJsonEvent returns a new event with the payload converted from CCF to JSON
func CcfEventToJsonEvent(e flow.Event) (*flow.Event, error) {
	convertedPayload, err := CcfPayloadToJsonPayload(e.Payload)
	if err != nil {
		return nil, err
	}
	return &flow.Event{
		Type:             e.Type,
		TransactionID:    e.TransactionID,
		TransactionIndex: e.TransactionIndex,
		EventIndex:       e.EventIndex,
		Payload:          convertedPayload,
	}, nil
}

// MessagesToBlockEvents converts a protobuf EventsResponse_Result messages to a slice of flow.BlockEvents.
func MessagesToBlockEvents(blocksEvents []*accessproto.EventsResponse_Result) []flow.BlockEvents {
	evs := make([]flow.BlockEvents, len(blocksEvents))
	for i, ev := range blocksEvents {
		evs[i] = MessageToBlockEvents(ev)
	}

	return evs
}

// MessageToBlockEvents converts a protobuf EventsResponse_Result message to a flow.BlockEvents.
func MessageToBlockEvents(blockEvents *accessproto.EventsResponse_Result) flow.BlockEvents {
	return flow.BlockEvents{
		BlockHeight:    blockEvents.BlockHeight,
		BlockID:        MessageToIdentifier(blockEvents.BlockId),
		BlockTimestamp: blockEvents.BlockTimestamp.AsTime(),
		Events:         MessagesToEvents(blockEvents.Events),
	}
}

func BlockEventsToMessages(blocks []flow.BlockEvents) ([]*accessproto.EventsResponse_Result, error) {
	results := make([]*accessproto.EventsResponse_Result, len(blocks))

	for i, block := range blocks {
		event, err := BlockEventsToMessage(block)
		if err != nil {
			return nil, err
		}
		results[i] = event
	}

	return results, nil
}

func BlockEventsToMessage(block flow.BlockEvents) (*accessproto.EventsResponse_Result, error) {
	eventMessages := make([]*entities.Event, len(block.Events))
	for i, event := range block.Events {
		eventMessages[i] = EventToMessage(event)
	}
	timestamp := timestamppb.New(block.BlockTimestamp)
	return &accessproto.EventsResponse_Result{
		BlockId:        block.BlockID[:],
		BlockHeight:    block.BlockHeight,
		BlockTimestamp: timestamp,
		Events:         eventMessages,
	}, nil
}
