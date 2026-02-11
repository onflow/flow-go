package fixtures

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/stdlib"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// Event is the default options factory for [flow.Event] generation.
var Event eventFactory

type eventFactory struct{}

type EventOption func(*EventGenerator, *eventConfig)

// eventConfig holds the configuration for event generation.
type eventConfig struct {
	event    flow.Event
	encoding entities.EventEncodingVersion
}

// WithEventType is an option that sets the event type for the event.
func (f eventFactory) WithEventType(eventType flow.EventType) EventOption {
	return func(g *EventGenerator, config *eventConfig) {
		config.event.Type = eventType
	}
}

// WithTransactionID is an option that sets the transaction ID for the event.
func (f eventFactory) WithTransactionID(transactionID flow.Identifier) EventOption {
	return func(g *EventGenerator, config *eventConfig) {
		config.event.TransactionID = transactionID
	}
}

// WithTransactionIndex is an option that sets the transaction index for the event.
func (f eventFactory) WithTransactionIndex(transactionIndex uint32) EventOption {
	return func(g *EventGenerator, config *eventConfig) {
		config.event.TransactionIndex = transactionIndex
	}
}

// WithEventIndex is an option that sets the event index for the event.
func (f eventFactory) WithEventIndex(eventIndex uint32) EventOption {
	return func(g *EventGenerator, config *eventConfig) {
		config.event.EventIndex = eventIndex
	}
}

// WithPayload is an option that sets the payload for the event.
// Note: if payload is provided, it must already be in the desired encoding.
func (f eventFactory) WithPayload(payload []byte) EventOption {
	return func(g *EventGenerator, config *eventConfig) {
		config.event.Payload = payload
	}
}

// WithEncoding is an option that sets the encoding for the event payload.
func (f eventFactory) WithEncoding(encoding entities.EventEncodingVersion) EventOption {
	return func(g *EventGenerator, config *eventConfig) {
		config.encoding = encoding
	}
}

// EventGenerator generates events with consistent randomness.
type EventGenerator struct {
	eventFactory

	random      *RandomGenerator
	identifiers *IdentifierGenerator
	eventTypes  *EventTypeGenerator
	addresses   *AddressGenerator
}

func NewEventGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
	eventTypes *EventTypeGenerator,
	addresses *AddressGenerator,
) *EventGenerator {
	return &EventGenerator{
		random:      random,
		identifiers: identifiers,
		eventTypes:  eventTypes,
		addresses:   addresses,
	}
}

// Fixture generates a [flow.Event] with random data based on the provided options.
func (g *EventGenerator) Fixture(opts ...EventOption) flow.Event {
	config := &eventConfig{
		event: flow.Event{
			Type:             g.eventTypes.Fixture(),
			TransactionID:    g.identifiers.Fixture(),
			TransactionIndex: 0,
			EventIndex:       0,
			Payload:          nil, // Will be generated based on encoding
		},
		encoding: entities.EventEncodingVersion_CCF_V0, // Default to CCF
	}

	for _, opt := range opts {
		opt(g, config)
	}

	// Generate payload if not provided
	if config.event.Payload == nil {
		config.event.Payload = g.generateEncodedPayload(config.event.Type, config.encoding)
	}

	return config.event
}

// List generates a list of [flow.Event].
func (g *EventGenerator) List(n int, opts ...EventOption) []flow.Event {
	list := make([]flow.Event, n)
	for i := range n {
		// For lists, we want sequential indices
		list[i] = g.Fixture(append(opts,
			Event.WithTransactionIndex(uint32(i)),
			Event.WithEventIndex(uint32(i)),
		)...)
	}
	// ensure event/transaction indexes are sequential
	list = AdjustEventsMetadata(list)
	return list
}

// ForTransaction generates a list of [flow.Event] for a specific transaction.
func (g *EventGenerator) ForTransaction(transactionID flow.Identifier, transactionIndex uint32, eventCount int, opts ...EventOption) []flow.Event {
	events := make([]flow.Event, eventCount)
	for i := range eventCount {
		eventOpts := append(opts,
			Event.WithTransactionID(transactionID),
			Event.WithTransactionIndex(transactionIndex),
			Event.WithEventIndex(uint32(i)),
		)
		events[i] = g.Fixture(eventOpts...)
	}
	return events
}

// ForTransactions generates a list of [flow.Event] for multiple transactions.
func (g *EventGenerator) ForTransactions(transactionIDs []flow.Identifier, eventsPerTransaction int, opts ...EventOption) []flow.Event {
	var allEvents []flow.Event
	for i, txID := range transactionIDs {
		txEvents := g.ForTransaction(txID, uint32(i), eventsPerTransaction, opts...)
		allEvents = append(allEvents, txEvents...)
	}
	return allEvents
}

// Helper methods for generating random values

// generateEncodedPayload generates a properly encoded event payload based on the specified encoding.
func (g *EventGenerator) generateEncodedPayload(eventType flow.EventType, encoding entities.EventEncodingVersion) []byte {
	testEvent := g.generateCadenceEvent(eventType)

	switch encoding {
	case entities.EventEncodingVersion_CCF_V0:
		payload, err := ccf.Encode(testEvent)
		NoError(err)
		return payload

	case entities.EventEncodingVersion_JSON_CDC_V0:
		payload, err := jsoncdc.Encode(testEvent)
		NoError(err)
		return payload

	default:
		// Fallback to random bytes for unknown encoding
		return g.random.RandomBytes(10)
	}
}

// generateCadenceEvent generates a cadence event fixture from a flow event type.
func (g *EventGenerator) generateCadenceEvent(eventType flow.EventType) cadence.Event {
	parsed, err := events.ParseEvent(eventType)
	NoError(err)

	var fields []cadence.Field
	var values []cadence.Value
	var cadenceEventType *cadence.EventType

	if parsed.Type == events.ProtocolEventType {
		fields, values = g.generateProtocolEventData(parsed.Name)
		cadenceEventType = cadence.NewEventType(
			stdlib.FlowLocation{},
			parsed.Name,
			fields,
			nil,
		)
	} else {
		address, err := common.BytesToAddress(flow.HexToAddress(parsed.Address).Bytes())
		NoError(err)

		fields, values = g.generateGenericEventData()
		cadenceEventType = cadence.NewEventType(
			common.NewAddressLocation(nil, address, parsed.ContractName),
			fmt.Sprintf("%s.%s", parsed.ContractName, parsed.Name),
			fields,
			nil,
		)
	}

	return cadence.NewEvent(values).WithType(cadenceEventType)
}

// generateGenericEventData generates generic event data for a cadence event.
func (g *EventGenerator) generateGenericEventData() (fields []cadence.Field, values []cadence.Value) {
	testString, err := cadence.NewString(g.random.RandomString(10))
	NoError(err)

	fields = []cadence.Field{
		{
			Identifier: "value",
			Type:       cadence.IntType,
		},
		{
			Identifier: "message",
			Type:       cadence.StringType,
		},
	}
	values = []cadence.Value{
		cadence.NewInt(g.random.IntInRange(1, 100)),
		testString,
	}

	return fields, values
}

// generateProtocolEventData generates protocol event data for a cadence event.
func (g *EventGenerator) generateProtocolEventData(eventName string) ([]cadence.Field, []cadence.Value) {
	switch eventName {
	case "AccountCreated":
		address, err := common.BytesToAddress(g.addresses.Fixture().Bytes())
		NoError(err)

		fields := []cadence.Field{
			{
				Identifier: "address",
				Type:       cadence.AddressType,
			},
		}
		values := []cadence.Value{
			cadence.NewAddress(address),
		}

		return fields, values

	case "AccountContractAdded":
		address, err := common.BytesToAddress(g.addresses.Fixture().Bytes())
		NoError(err)

		contractName, err := cadence.NewString("EventContract")
		NoError(err)

		codeHash := unittest.BytesToCdcUInt8(g.random.RandomBytes(32))

		fields := []cadence.Field{
			{
				Identifier: "address",
				Type:       cadence.AddressType,
			},
			{
				Identifier: "codeHash",
				Type:       cadence.NewConstantSizedArrayType(32, cadence.UInt8Type),
			},
			{
				Identifier: "contract",
				Type:       cadence.StringType,
			},
		}
		values := []cadence.Value{
			cadence.NewAddress(address),
			cadence.NewArray(codeHash).WithType(cadence.NewConstantSizedArrayType(32, cadence.UInt8Type)),
			contractName,
		}

		return fields, values

	default:
		// If you get this error, you will need to add support for the new event type using the correct fields and values.
		panic(fmt.Sprintf("unsupported protocol event type: flow.%s", eventName))
	}
}

// AdjustEventsMetadata adjusts the event and transaction indexes to be sequential.
// The following changes are made:
// - Transaction Index is updated to match the actual transactions
// - Event Index is updated to be sequential and reset for each transaction
func AdjustEventsMetadata(events []flow.Event) []flow.Event {
	if len(events) == 0 {
		return events
	}

	lastTxID := events[0].TransactionID
	txIndex := events[0].TransactionIndex
	eventIndex := uint32(0)

	output := make([]flow.Event, len(events))
	for i, event := range events {
		if event.TransactionID != lastTxID {
			lastTxID = event.TransactionID
			txIndex++
			eventIndex = 0
		}

		event.EventIndex = eventIndex
		event.TransactionIndex = txIndex
		eventIndex++

		output[i] = event
	}
	return output
}
