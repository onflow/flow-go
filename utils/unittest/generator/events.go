package generator

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/testutils"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/flow/protobuf/go/flow/entities"

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

func (g *Events) New() flow.Event {
	address, err := common.BytesToAddress(unittest.RandomAddressFixture().Bytes())
	if err != nil {
		panic(fmt.Sprintf("unexpected error while creating random address: %s", err))
	}

	location := common.NewAddressLocation(nil, address, "TestContract")
	identifier := fmt.Sprintf("TestContract.FooEvent%d", g.count)
	typeID := location.TypeID(nil, identifier)

	testEventType := &cadence.EventType{
		Location:            location,
		QualifiedIdentifier: identifier,
		Fields: []cadence.Field{
			{
				Identifier: "a",
				Type:       cadence.IntType{},
			},
			{
				Identifier: "b",
				Type:       cadence.StringType{},
			},
		},
	}

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

	event := flow.Event{
		Type:             flow.EventType(typeID),
		TransactionID:    g.ids.New(),
		TransactionIndex: g.count,
		EventIndex:       g.count,
		Payload:          payload,
	}

	g.count++

	return event
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
		}).WithType(&cadence.EventType{
		Location:            stdlib.FlowLocation{},
		QualifiedIdentifier: "AccountCreated",
		Fields: []cadence.Field{
			{
				Identifier: "address",
				Type:       cadence.AddressType{},
			},
		},
	})

	payload, err := ccf.Encode(cadenceEvent)
	require.NoError(t, err)

	event := unittest.EventFixture(
		flow.EventType(cadenceEvent.EventType.Location.TypeID(nil, cadenceEvent.EventType.QualifiedIdentifier)),
		0,
		0,
		unittest.IdentifierFixture(),
		0,
	)

	event.Payload = payload

	return event
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
			).WithType(cadence.NewConstantSizedArrayType(32, cadence.TheUInt8Type)),
			contractName,
		}).WithType(&cadence.EventType{
		Location:            stdlib.FlowLocation{},
		QualifiedIdentifier: qualifiedIdentifier,
		Fields: []cadence.Field{
			{
				Identifier: "address",
				Type:       cadence.AddressType{},
			},
			{
				Identifier: "codeHash",
				Type:       cadence.NewConstantSizedArrayType(32, cadence.TheUInt8Type),
			},
			{
				Identifier: "contract",
				Type:       cadence.StringType{},
			},
		},
	})

	payload, err := ccf.Encode(cadenceEvent)
	require.NoError(t, err)

	event := unittest.EventFixture(
		flow.EventType(cadenceEvent.EventType.Location.TypeID(nil, cadenceEvent.EventType.QualifiedIdentifier)),
		0,
		0,
		unittest.IdentifierFixture(),
		0,
	)

	event.Payload = payload

	return event
}
