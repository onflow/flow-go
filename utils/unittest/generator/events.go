package generator

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/flow"
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
	location := common.StringLocation("test")
	identifier := fmt.Sprintf("FooEvent%d", g.count)
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
