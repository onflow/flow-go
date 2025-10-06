package inmemory

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type EventsReader struct {
	blockID flow.Identifier
	events  []flow.Event
}

var _ storage.EventsReader = (*EventsReader)(nil)

func NewEvents(blockID flow.Identifier, events []flow.Event) *EventsReader {
	return &EventsReader{
		blockID: blockID,
		events:  events,
	}
}

// ByBlockID returns the events for the given block ID.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if no events were found at given block.
func (e *EventsReader) ByBlockID(blockID flow.Identifier) ([]flow.Event, error) {
	if e.blockID != blockID {
		return nil, storage.ErrNotFound
	}

	return e.events, nil
}

// ByBlockIDTransactionID returns the events for the given block ID and transaction ID.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if no events were found at given block and transaction.
func (e *EventsReader) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) ([]flow.Event, error) {
	if e.blockID != blockID {
		return nil, storage.ErrNotFound
	}

	var matched []flow.Event
	for _, event := range e.events {
		if event.TransactionID == txID {
			matched = append(matched, event)
		}
	}

	return matched, nil
}

// ByBlockIDTransactionIndex returns the events for the transaction at given index in a given block
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if no events were found at given block and transaction.
func (e *EventsReader) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) ([]flow.Event, error) {
	if e.blockID != blockID {
		return nil, storage.ErrNotFound
	}

	var matched []flow.Event
	for _, event := range e.events {
		if event.TransactionIndex == txIndex {
			matched = append(matched, event)
		}
	}

	return matched, nil
}

// ByBlockIDEventType returns the events for the given block ID and event type.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if no events were found at given block.
func (e *EventsReader) ByBlockIDEventType(blockID flow.Identifier, eventType flow.EventType) ([]flow.Event, error) {
	if e.blockID != blockID {
		return nil, storage.ErrNotFound
	}

	var matched []flow.Event
	for _, event := range e.events {
		if event.Type == eventType {
			matched = append(matched, event)
		}
	}

	return matched, nil
}
