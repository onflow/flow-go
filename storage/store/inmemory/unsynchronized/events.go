package unsynchronized

import (
	"fmt"
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

var _ storage.Events = (*Events)(nil)

func NewEvents() *Events {
	return &Events{
		blockIdToEvents: make(map[flow.Identifier][]flow.Event),
	}
}

// ByBlockID returns the events for the given block ID.
// Returns storage.ErrNotFound if block wasn't found.
func (e *Events) ByBlockID(blockID flow.Identifier) ([]flow.Event, error) {
	e.lock.RLock()
	defer e.lock.RUnlock()

	val, ok := e.blockIdToEvents[blockID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// ByBlockIDTransactionID returns the events for the given block ID and transaction ID.
// Returns storage.ErrNotFound if block wasn't found.
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

// ByBlockIDTransactionIndex returns the events for the transaction at given index in a given block
// Returns storage.ErrNotFound if block wasn't found.
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

// ByBlockIDEventType returns the events for the given block ID and event type.
// Returns storage.ErrNotFound if block wasn't found in storage.
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

// Store will store events for the given block ID.
// No errors expected during normal operation.
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

// BatchStore will store events for the given block ID in a given batch
func (e *Events) BatchStore(flow.Identifier, []flow.EventsList, storage.ReaderBatchWriter) error {
	return fmt.Errorf("not implemented")
}

// BatchRemoveByBlockID removes events keyed by a blockID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If database unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (e *Events) BatchRemoveByBlockID(flow.Identifier, storage.ReaderBatchWriter) error {
	return fmt.Errorf("not implemented")
}
