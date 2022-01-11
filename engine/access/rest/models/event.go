package models

import "github.com/onflow/flow-go/model/flow"

func (e *Event) Build(event flow.Event) {
	e.Type_ = string(event.Type)
	e.TransactionId = event.TransactionID.String()
	e.TransactionIndex = fromUint64(uint64(event.TransactionIndex))
	e.EventIndex = fromUint64(uint64(event.EventIndex))
	e.Payload = ToBase64(event.Payload)
}

type Events []Event

func (e *Events) Build(events []flow.Event) {
	evs := make([]Event, len(events))
	for i, ev := range events {
		var event Event
		event.Build(ev)
		evs[i] = event
	}

	*e = evs
}
