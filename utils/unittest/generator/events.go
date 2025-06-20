package generator

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
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type EventGeneratorOption func(*Events)

func WithEncoding(encoding entities.EventEncodingVersion) EventGeneratorOption {
	return func(g *Events) {
		g.encoding = encoding
	}
}

type Events struct {
	count    uint32
	ids      *Identifiers
	encoding entities.EventEncodingVersion
}

func EventGenerator(opts ...EventGeneratorOption) *Events {
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

var Event eventFactory

type eventFactory struct{}

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

func (g *Events) New(opts ...EventOption) flow.Event {
	address, err := common.BytesToAddress(unittest.RandomAddressFixture().Bytes())
	if err != nil {
		panic(fmt.Sprintf("unexpected error while creating random address: %s", err))
	}

	location := common.NewAddressLocation(nil, address, "TestContract")
	identifier := fmt.Sprintf("TestContract.FooEvent%d", g.count)
	typeID := location.TypeID(nil, identifier)

	event := &flow.Event{
		Type:             flow.EventType(typeID),
		TransactionID:    g.ids.New(),
		TransactionIndex: g.count,
		EventIndex:       g.count,
		Payload:          g.createNewEventPayload(location, identifier),
	}

	for _, opt := range opts {
		opt(event)
	}
	g.count++

	return *event
}

func (g *Events) createNewEventPayload(location common.AddressLocation, identifier string) []byte {
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

	fooString, err := cadence.NewString("foo")
	if err != nil {
		panic(fmt.Sprintf("unexpected error while creating cadence string: %s", err))
	}

	testEvent := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewInt(int(g.count)),
			fooString,
		}).WithType(testEventType)

	var payload []byte
	switch g.encoding {
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

// GetEventsWithEncoding generates a specified number of events with a given encoding version.
func GetEventsWithEncoding(n int, version entities.EventEncodingVersion) []flow.Event {
	eventGenerator := EventGenerator(WithEncoding(version))
	events := make([]flow.Event, 0, n)
	for i := 0; i < n; i++ {
		events = append(events, eventGenerator.New())
	}
	return events
}

// GenerateAccountCreateEvent returns a mock account creation event.
func GenerateAccountCreateEvent(t *testing.T, address flow.Address) flow.Event {
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
		TransactionID:    unittest.IdentifierFixture(),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          payload,
	}
}

// GenerateAccountContractEvent returns a mock account contract event.
func GenerateAccountContractEvent(t *testing.T, qualifiedIdentifier string, address flow.Address) flow.Event {
	contractName, err := cadence.NewString("EventContract")
	require.NoError(t, err)

	cadenceEvent := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewAddress(address),
			cadence.NewArray(
				testutils.ConvertToCadence([]byte{111, 43, 164, 202, 220, 174, 148, 17, 253, 161, 9, 124, 237, 83, 227, 75, 115, 149, 141, 83, 129, 145, 252, 68, 122, 137, 80, 155, 89, 233, 136, 213}),
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
		TransactionID:    unittest.IdentifierFixture(),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          payload,
	}
}

// EventFixture returns a single event
func EventFixture(
	opts ...EventOption,
) flow.Event {
	g := EventGenerator(WithEncoding(entities.EventEncodingVersion_CCF_V0))
	return g.New(opts...)
}

func EventsFixture(
	n int,
) []flow.Event {
	events := make([]flow.Event, n)
	g := EventGenerator(WithEncoding(entities.EventEncodingVersion_CCF_V0))
	for i := 0; i < n; i++ {
		events[i] = g.New(
			Event.WithTransactionIndex(0),
			Event.WithEventIndex(uint32(i)),
		)
	}

	return events
}

// BlockEventsFixture returns a block events model populated with random events of length n.
func BlockEventsFixture(
	header *flow.Header,
	n int,
) flow.BlockEvents {
	return flow.BlockEvents{
		BlockID:        header.ID(),
		BlockHeight:    header.Height,
		BlockTimestamp: header.Timestamp,
		Events:         EventsFixture(n),
	}
}
