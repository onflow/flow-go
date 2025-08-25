package unittest

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/stdlib"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/flow"
)

// EventGeneratorOption configures an Events generator.
type EventGeneratorOption func(*Events)

// Events generates mock Flow events with incremental count and optional encoding.
type Events struct {
	count    uint32
	ids      *Identifiers
	encoding entities.EventEncodingVersion
}

// NewEventGenerator creates a new Events generator.
func NewEventGenerator(opts ...EventGeneratorOption) *Events {
	g := &Events{
		count:    1,
		ids:      IdentifierGenerator(),
		encoding: entities.EventEncodingVersion_CCF_V0,
	}

	for _, opt := range opts {
		opt(g)
	}

	return g
}

// New creates a new flow.Event.
func (e *Events) New(opts ...EventOption) flow.Event {
	address, err := common.BytesToAddress(RandomAddressFixture().Bytes())
	if err != nil {
		panic(fmt.Sprintf("unexpected error while creating random address: %s", err))
	}

	location := common.NewAddressLocation(nil, address, "TestContract")
	identifier := fmt.Sprintf("TestContract.FooEvent%d", e.count)
	typeID := location.TypeID(nil, identifier)

	event := &flow.Event{
		Type:             flow.EventType(typeID),
		TransactionID:    e.ids.New(),
		TransactionIndex: e.count,
		EventIndex:       e.count,
		Payload:          e.createNewEventPayload(location, identifier),
	}

	for _, opt := range opts {
		opt(event)
	}
	e.count++

	return *event
}

func (e *Events) createNewEventPayload(location common.AddressLocation, identifier string) []byte {
	testEventType := cadence.NewEventType(
		location,
		identifier,
		[]cadence.Field{
			{
				Identifier: "a",
				Type:       cadence.IntType,
			},
			{
				Identifier: "b",
				Type:       cadence.StringType,
			},
		},
		nil,
	)

	randomString := cadence.String(hex.EncodeToString(RandomBytes(100)))

	testEvent := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewInt(int(e.count)),
			randomString,
		}).WithType(testEventType)

	var payload []byte
	var err error
	switch e.encoding {
	case entities.EventEncodingVersion_CCF_V0:
		payload, err = ccf.Encode(testEvent)
		if err != nil {
			panic(fmt.Sprintf("unexpected error while ccf encoding events: %s", err))
		}
	case entities.EventEncodingVersion_JSON_CDC_V0:
		payload, err = jsoncdc.Encode(testEvent)
		if err != nil {
			panic(fmt.Sprintf("unexpected error while json encoding events: %s", err))
		}
	}

	return payload
}

var EventGenerator eventGeneratorFactory

type eventGeneratorFactory struct{}

// WithEncoding sets event encoding (CCF or JSON).
func (e *eventGeneratorFactory) WithEncoding(encoding entities.EventEncodingVersion) EventGeneratorOption {
	return func(g *Events) {
		g.encoding = encoding
	}
}

// GetEventsWithEncoding generates a specified number of events with a given encoding version.
func (e *eventGeneratorFactory) GetEventsWithEncoding(n int, version entities.EventEncodingVersion) []flow.Event {
	eventGenerator := NewEventGenerator(EventGenerator.WithEncoding(version))
	events := make([]flow.Event, 0, n)
	for i := 0; i < n; i++ {
		events = append(events, eventGenerator.New())
	}
	return events
}

// GenerateAccountCreateEvent returns a mock account creation event.
func (e *eventGeneratorFactory) GenerateAccountCreateEvent(t *testing.T, address flow.Address) flow.Event {
	cadenceEvent := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewAddress(address),
		}).
		WithType(cadence.NewEventType(
			stdlib.FlowLocation{},
			"AccountCreated",
			[]cadence.Field{
				{
					Identifier: "address",
					Type:       cadence.AddressType,
				},
			},
			nil,
		))

	payload, err := ccf.Encode(cadenceEvent)
	require.NoError(t, err)

	return flow.Event{
		Type:             flow.EventType(cadenceEvent.EventType.Location.TypeID(nil, cadenceEvent.EventType.QualifiedIdentifier)),
		TransactionID:    IdentifierFixture(),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          payload,
	}
}

// GenerateAccountContractEvent returns a mock account contract event.
func (e *eventGeneratorFactory) GenerateAccountContractEvent(t *testing.T, qualifiedIdentifier string, address flow.Address) flow.Event {
	contractName, err := cadence.NewString("EventContract")
	require.NoError(t, err)

	cadenceEvent := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewAddress(address),
			cadence.NewArray(
				BytesToCdcUInt8(RandomBytes(32)),
			).WithType(cadence.NewConstantSizedArrayType(32, cadence.UInt8Type)),
			contractName,
		}).
		WithType(cadence.NewEventType(
			stdlib.FlowLocation{},
			qualifiedIdentifier,
			[]cadence.Field{
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
			},
			nil,
		))

	payload, err := ccf.Encode(cadenceEvent)
	require.NoError(t, err)

	return flow.Event{
		Type:             flow.EventType(cadenceEvent.EventType.Location.TypeID(nil, cadenceEvent.EventType.QualifiedIdentifier)),
		TransactionID:    IdentifierFixture(),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          payload,
	}
}

var Event eventFactory

type eventFactory struct{}

// EventOption configures a flow.Event fields.
type EventOption func(*flow.Event)

func (f *eventFactory) WithEventType(eventType flow.EventType) EventOption {
	return func(e *flow.Event) {
		e.Type = eventType
	}
}

func (f *eventFactory) WithTransactionIndex(transactionIndex uint32) EventOption {
	return func(e *flow.Event) {
		e.TransactionIndex = transactionIndex
	}
}

func (f *eventFactory) WithEventIndex(eventIndex uint32) EventOption {
	return func(e *flow.Event) {
		e.EventIndex = eventIndex
	}
}

func (f *eventFactory) WithTransactionID(txID flow.Identifier) EventOption {
	return func(e *flow.Event) {
		e.TransactionID = txID
	}
}

func (f *eventFactory) WithPayload(payload []byte) EventOption {
	return func(e *flow.Event) {
		e.Payload = payload
	}
}

// EventFixture returns a single event
func EventFixture(
	opts ...EventOption,
) flow.Event {
	g := NewEventGenerator(EventGenerator.WithEncoding(entities.EventEncodingVersion_CCF_V0))
	return g.New(opts...)
}

func EventsFixture(
	n int,
) []flow.Event {
	events := make([]flow.Event, n)
	g := NewEventGenerator(EventGenerator.WithEncoding(entities.EventEncodingVersion_CCF_V0))
	for i := 0; i < n; i++ {
		events[i] = g.New(
			Event.WithTransactionIndex(0),
			Event.WithEventIndex(uint32(i)),
		)
	}

	return events
}
