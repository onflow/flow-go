package fixtures

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/stdlib"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
)

// EventGenerator generates events with consistent randomness.
type EventGenerator struct {
	randomGen     *RandomGenerator
	identifierGen *IdentifierGenerator
	eventTypeGen  *EventTypeGenerator
	addressGen    *AddressGenerator
}

// eventConfig holds the configuration for event generation.
type eventConfig struct {
	eventType        flow.EventType
	transactionID    flow.Identifier
	transactionIndex uint32
	eventIndex       uint32
	payload          []byte
	encoding         entities.EventEncodingVersion
}

// WithEventType returns an option to set the event type for the event.
func (g *EventGenerator) WithEventType(eventType flow.EventType) func(*eventConfig) {
	return func(config *eventConfig) {
		config.eventType = eventType
	}
}

// WithTransactionID returns an option to set the transaction ID for the event.
func (g *EventGenerator) WithTransactionID(transactionID flow.Identifier) func(*eventConfig) {
	return func(config *eventConfig) {
		config.transactionID = transactionID
	}
}

// WithTransactionIndex returns an option to set the transaction index for the event.
func (g *EventGenerator) WithTransactionIndex(transactionIndex uint32) func(*eventConfig) {
	return func(config *eventConfig) {
		config.transactionIndex = transactionIndex
	}
}

// WithEventIndex returns an option to set the event index for the event.
func (g *EventGenerator) WithEventIndex(eventIndex uint32) func(*eventConfig) {
	return func(config *eventConfig) {
		config.eventIndex = eventIndex
	}
}

// WithPayload returns an option to set the payload for the event.
// Note: if payload is provided, it must already be in the desired encoding.
func (g *EventGenerator) WithPayload(payload []byte) func(*eventConfig) {
	return func(config *eventConfig) {
		config.payload = payload
	}
}

// WithEncoding returns an option to set the encoding for the event payload.
func (g *EventGenerator) WithEncoding(encoding entities.EventEncodingVersion) func(*eventConfig) {
	return func(config *eventConfig) {
		config.encoding = encoding
	}
}

// Fixture generates an event with optional configuration.
func (g *EventGenerator) Fixture(t testing.TB, opts ...func(*eventConfig)) flow.Event {
	config := &eventConfig{
		eventType:        g.eventTypeGen.Fixture(t),
		transactionID:    g.identifierGen.Fixture(t),
		transactionIndex: 0,
		eventIndex:       0,
		payload:          nil,                                  // Will be generated based on encoding
		encoding:         entities.EventEncodingVersion_CCF_V0, // Default to CCF
	}

	for _, opt := range opts {
		opt(config)
	}

	// Generate payload if not provided
	if config.payload == nil {
		config.payload = g.generateEncodedPayload(t, config.eventType, config.encoding)
	}

	return flow.Event{
		Type:             config.eventType,
		TransactionID:    config.transactionID,
		TransactionIndex: config.transactionIndex,
		EventIndex:       config.eventIndex,
		Payload:          config.payload,
	}
}

// List generates a list of events.
func (g *EventGenerator) List(t testing.TB, n int, opts ...func(*eventConfig)) []flow.Event {
	list := make([]flow.Event, n)
	for i := range n {
		// For lists, we want sequential indices
		list[i] = g.Fixture(t, append(opts, g.WithTransactionIndex(uint32(i)), g.WithEventIndex(uint32(i)))...)
	}
	// ensure event/transaction indexes are sequential
	list = AdjustEventsMetadata(list)
	return list
}

// ForTransaction generates events for a specific transaction.
func (g *EventGenerator) ForTransaction(t testing.TB, transactionID flow.Identifier, transactionIndex uint32, eventCount int, opts ...func(*eventConfig)) []flow.Event {
	events := make([]flow.Event, eventCount)
	for i := range eventCount {
		eventOpts := append(opts,
			g.WithTransactionID(transactionID),
			g.WithTransactionIndex(transactionIndex),
			g.WithEventIndex(uint32(i)),
		)
		events[i] = g.Fixture(t, eventOpts...)
	}
	return events
}

// ForTransactions generates events for multiple transactions.
func (g *EventGenerator) ForTransactions(t testing.TB, transactionIDs []flow.Identifier, eventsPerTransaction int, opts ...func(*eventConfig)) []flow.Event {
	var allEvents []flow.Event
	for i, txID := range transactionIDs {
		txEvents := g.ForTransaction(t, txID, uint32(i), eventsPerTransaction, opts...)
		allEvents = append(allEvents, txEvents...)
	}
	return allEvents
}

// Helper methods for generating random values

// generateEncodedPayload generates a properly encoded event payload based on the specified encoding.
func (g *EventGenerator) generateEncodedPayload(t testing.TB, eventType flow.EventType, encoding entities.EventEncodingVersion) []byte {
	testEvent := g.generateCadenceEvent(t, eventType)

	switch encoding {
	case entities.EventEncodingVersion_CCF_V0:
		payload, err := ccf.Encode(testEvent)
		require.NoError(t, err)
		return payload

	case entities.EventEncodingVersion_JSON_CDC_V0:
		payload, err := jsoncdc.Encode(testEvent)
		require.NoError(t, err)
		return payload

	default:
		// Fallback to random bytes for unknown encoding
		return g.randomGen.RandomBytes(t, 10)
	}
}

// generateCadenceEvent generates a cadence event fixture from a flow event type.
func (g *EventGenerator) generateCadenceEvent(t testing.TB, eventType flow.EventType) cadence.Event {
	parsed, err := events.ParseEvent(eventType)
	require.NoError(t, err)

	var fields []cadence.Field
	var values []cadence.Value
	var cadenceEventType *cadence.EventType

	if parsed.Type == events.ProtocolEventType {
		fields, values = g.generateProtocolEventData(t, parsed.Name)
		cadenceEventType = cadence.NewEventType(
			stdlib.FlowLocation{},
			parsed.Name,
			fields,
			nil,
		)
	} else {
		address, err := common.BytesToAddress(flow.HexToAddress(parsed.Address).Bytes())
		require.NoError(t, err)

		fields, values = g.generateGenericEventData(t)
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
func (g *EventGenerator) generateGenericEventData(t testing.TB) (fields []cadence.Field, values []cadence.Value) {
	testString, err := cadence.NewString(g.randomGen.RandomString(10))
	require.NoError(t, err)

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
		cadence.NewInt(int(g.randomGen.Uint64InRange(1, 100))),
		testString,
	}

	return fields, values
}

// generateProtocolEventData generates protocol event data for a cadence event.
func (g *EventGenerator) generateProtocolEventData(t testing.TB, eventName string) ([]cadence.Field, []cadence.Value) {
	switch eventName {
	case "AccountCreated":
		address, err := common.BytesToAddress(g.addressGen.Fixture(t).Bytes())
		require.NoError(t, err)

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
		address, err := common.BytesToAddress(g.addressGen.Fixture(t).Bytes())
		require.NoError(t, err)

		contractName, err := cadence.NewString("EventContract")
		require.NoError(t, err)

		codeHash := testutils.ConvertToCadence(g.randomGen.RandomBytes(t, 32))

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
		t.Fatalf("unsupported protocol event type: flow.%s", eventName)
		return nil, nil
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
	txIndex := uint32(0)
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
