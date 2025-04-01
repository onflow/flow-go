package unsynchronized

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type Events struct {
	//TODO: we don't need a mutex here as we have a guarantee by design
	// that we write data only once and it happens before the future reads.
	// We decided to leave a mutex for some time during active development.
	// It'll be removed in the future.
	lock            sync.RWMutex
	blockIdToEvents map[flow.Identifier][]flow.Event
}

func NewEvents() *Events {
	return &Events{
		blockIdToEvents: make(map[flow.Identifier][]flow.Event),
	}
}

var _ storage.EventsReader = (*Events)(nil)

func (e *Events) ByBlockID(blockID flow.Identifier) ([]flow.Event, error) {
	e.lock.RLock()
	val, ok := e.blockIdToEvents[blockID]
	e.lock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

func (e *Events) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) ([]flow.Event, error) {
	events, err := e.ByBlockID(blockID)
	if err != nil {
		return nil, storage.ErrNotFound
	}

	var matched []flow.Event
	for _, event := range events {
		if event.TransactionID == txID {
			matched = append(matched, event)
		}
	}

	return matched, nil
}

func (e *Events) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) ([]flow.Event, error) {
	events, err := e.ByBlockID(blockID)
	if err != nil {
		return nil, storage.ErrNotFound
	}

	var matched []flow.Event
	for _, event := range events {
		if event.TransactionIndex == txIndex {
			matched = append(matched, event)
		}
	}

	return matched, nil
}

func (e *Events) ByBlockIDEventType(blockID flow.Identifier, eventType flow.EventType) ([]flow.Event, error) {
	events, err := e.ByBlockID(blockID)
	if err != nil {
		return nil, storage.ErrNotFound
	}

	var matched []flow.Event
	for _, event := range events {
		if event.Type == eventType {
			matched = append(matched, event)
		}
	}

	return matched, nil
}

func (e *Events) Store(blockID flow.Identifier, blockEvents []flow.EventsList) error {
	var events []flow.Event
	for _, eventList := range blockEvents {
		events = append(events, eventList...)
	}

	e.lock.Lock()
	e.blockIdToEvents[blockID] = events
	e.lock.Unlock()

	return nil
}
