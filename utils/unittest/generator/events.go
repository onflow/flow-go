package generator

import (
	"fmt"

	"github.com/onflow/cadence"
	encoding "github.com/onflow/cadence/encoding/json"

	"github.com/dapperlabs/flow-go/model/flow"
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
	identifier := fmt.Sprintf("FooEvent%d", g.count)
	typeID := "test." + identifier

	testEventType := cadence.EventType{
		TypeID:     typeID,
		Identifier: identifier,
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

	testEvent := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewInt(int(g.count)),
			cadence.NewString("foo"),
		}).WithType(testEventType)

	payload, err := encoding.Encode(testEvent)
	if err != nil {
		panic(fmt.Sprintf("unexpected error while encoding events "))
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
