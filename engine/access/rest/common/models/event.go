package models

import (
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

func NewEvent(event flow.Event) Event {
	return Event{
		Type_:            string(event.Type),
		TransactionId:    event.TransactionID.String(),
		TransactionIndex: util.FromUint(uint64(event.TransactionIndex)),
		EventIndex:       util.FromUint(uint64(event.EventIndex)),
		Payload:          util.ToBase64(event.Payload),
	}
}

type Events []Event

func NewEvents(events []flow.Event) Events {
	convertedEvents := make([]Event, len(events))
	for i, event := range events {
		convertedEvents[i] = NewEvent(event)
	}
	return convertedEvents
}

func NewBlockEvents(
	events flow.BlockEvents,
	metadata *access.ExecutorMetadata,
	shouldIncludeMetadata bool,
) *BlockEvents {
	var meta *Metadata
	if shouldIncludeMetadata {
		meta = NewMetadata(metadata)
	}

	return &BlockEvents{
		BlockId:        events.BlockID.String(),
		BlockHeight:    util.FromUint(events.BlockHeight),
		BlockTimestamp: events.BlockTimestamp,
		Events:         NewEvents(events.Events),
		Metadata:       meta,
	}
}

type BlockEventsList []BlockEvents

func NewBlockEventsList(
	blocksEvents []flow.BlockEvents,
	metadata *access.ExecutorMetadata,
	shouldIncludeMetadata bool,
) BlockEventsList {
	converted := make([]BlockEvents, len(blocksEvents))
	for i, evs := range blocksEvents {
		converted[i] = *NewBlockEvents(evs, metadata, shouldIncludeMetadata)
	}

	return converted
}
