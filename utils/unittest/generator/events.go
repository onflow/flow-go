package generator

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/model/flow"
)

type EventEncoding string

const (
	EncodingCCF  EventEncoding = "ccf"
	EncodingJSON EventEncoding = "json"
)

type EventGeneratorOption func(*Events)

func WithEncoding(encoding EventEncoding) EventGeneratorOption {
	return func(g *Events) {
		if encoding != EncodingCCF && encoding != EncodingJSON {
			panic(fmt.Sprintf("unexpected encoding: %s", encoding))
		}

		g.encoding = encoding
	}
}

type Events struct {
	count    uint32
	ids      *Identifiers
	encoding EventEncoding
}

func EventGenerator(opts ...EventGeneratorOption) *Events {
	g := &Events{
		count:    1,
		ids:      IdentifierGenerator(),
		encoding: EncodingCCF,
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
	case EncodingCCF:
		payload, err = ccf.Encode(testEvent)
		if err != nil {
			panic(fmt.Sprintf("unexpected error while ccf encoding events: %s", err))
		}
	case EncodingJSON:
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
