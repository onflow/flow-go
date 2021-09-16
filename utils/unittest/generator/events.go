package generator

import (
	"fmt"

	"github.com/onflow/cadence"
	encoding "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/model/flow"
)

type Events struct {
	count uint32
	ids   *Identifiers
}

func EventGenerator() *Events {
	return &Events{
		count: 1,
		ids:   IdentifierGenerator(),
	}
}

func (g *Events) New() flow.Event {
	location := common.StringLocation("test")
	identifier := fmt.Sprintf("FooEvent%d", g.count)
	typeID := location.TypeID(identifier)

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
		panic(err)
	}
	testEvent := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewInt(int(g.count)),
			fooString,
		}).WithType(testEventType)

	payload, err := encoding.Encode(testEvent)
	if err != nil {
		panic(fmt.Sprintf("unexpected error while encoding events: %s", err))
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
