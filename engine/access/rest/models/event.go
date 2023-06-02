package models

import (
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

func (e *Event) Build(event flow.Event) {
	e.Type_ = string(event.Type)
	e.TransactionId = event.TransactionID.String()
	e.TransactionIndex = util.FromUint64(uint64(event.TransactionIndex))
	e.EventIndex = util.FromUint64(uint64(event.EventIndex))
	e.Payload = util.ToBase64(event.Payload)
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

func (b *BlockEvents) Build(blockEvents flow.BlockEvents) {
	b.BlockHeight = util.FromUint64(blockEvents.BlockHeight)
	b.BlockId = blockEvents.BlockID.String()
	b.BlockTimestamp = blockEvents.BlockTimestamp

	var events Events
	events.Build(blockEvents.Events)
	b.Events = events
}

type BlocksEvents []BlockEvents

func (b *BlocksEvents) Build(blocksEvents []flow.BlockEvents) {
	evs := make([]BlockEvents, 0)
	for _, ev := range blocksEvents {
		var blockEvent BlockEvents
		blockEvent.Build(ev)
		evs = append(evs, blockEvent)
	}

	*b = evs
}
