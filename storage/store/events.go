package store

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type Events struct {
	db    storage.DB
	cache *Cache[flow.Identifier, []flow.Event]
}

var _ storage.Events = (*Events)(nil)

func NewEvents(collector module.CacheMetrics, db storage.DB) *Events {
	retrieve := func(r storage.Reader, blockID flow.Identifier) ([]flow.Event, error) {
		var events []flow.Event
		err := operation.LookupEventsByBlockID(r, blockID, &events)
		return events, err
	}

	return &Events{
		db: db,
		cache: newCache(collector, metrics.ResourceEvents,
			withStore(noopStore[flow.Identifier, []flow.Event]),
			withRetrieve(retrieve)),
	}
}

// BatchStore stores events keyed by a blockID in provided batch
// No errors are expected during normal operation, but it may return generic error
// if badger fails to process request
func (e *Events) BatchStore(blockID flow.Identifier, blockEvents []flow.EventsList, batch storage.ReaderBatchWriter) error {
	writer := batch.Writer()

	// pre-allocating and indexing slice is faster than appending
	sliceSize := 0
	for _, b := range blockEvents {
		sliceSize += len(b)
	}

	combinedEvents := make([]flow.Event, sliceSize)

	eventIndex := 0

	for _, events := range blockEvents {
		for _, event := range events {
			err := operation.InsertEvent(writer, blockID, event)
			if err != nil {
				return fmt.Errorf("cannot batch insert event: %w", err)
			}
			combinedEvents[eventIndex] = event
			eventIndex++
		}
	}

	callback := func() {
		e.cache.Insert(blockID, combinedEvents)
	}
	storage.OnCommitSucceed(batch, callback)
	return nil
}

// Store will store events for the given block ID
func (e *Events) Store(blockID flow.Identifier, blockEvents []flow.EventsList) error {
	return e.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return e.BatchStore(blockID, blockEvents, rw)
	})
}

// ByBlockID returns the events for the given block ID
// Note: This method will return an empty slice and no error if no entries for the blockID are found
func (e *Events) ByBlockID(blockID flow.Identifier) ([]flow.Event, error) {
	val, err := e.cache.Get(e.db.Reader(), blockID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// ByBlockIDTransactionID returns the events for the given block ID and transaction ID
// Note: This method will return an empty slice and no error if no entries for the blockID are found
func (e *Events) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) ([]flow.Event, error) {
	events, err := e.ByBlockID(blockID)
	if err != nil {
		return nil, err
	}

	var matched []flow.Event
	for _, event := range events {
		if event.TransactionID == txID {
			matched = append(matched, event)
		}
	}
	return matched, nil
}

// ByBlockIDTransactionIndex returns the events for the given block ID and transaction index
// Note: This method will return an empty slice and no error if no entries for the blockID are found
func (e *Events) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) ([]flow.Event, error) {
	events, err := e.ByBlockID(blockID)
	if err != nil {
		return nil, err
	}

	var matched []flow.Event
	for _, event := range events {
		if event.TransactionIndex == txIndex {
			matched = append(matched, event)
		}
	}
	return matched, nil
}

// ByBlockIDEventType returns the events for the given block ID and event type
// Note: This method will return an empty slice and no error if no entries for the blockID are found
func (e *Events) ByBlockIDEventType(blockID flow.Identifier, eventType flow.EventType) ([]flow.Event, error) {
	events, err := e.ByBlockID(blockID)
	if err != nil {
		return nil, err
	}

	var matched []flow.Event
	for _, event := range events {
		if event.Type == eventType {
			matched = append(matched, event)
		}
	}
	return matched, nil
}

// RemoveByBlockID removes events by block ID
func (e *Events) RemoveByBlockID(blockID flow.Identifier) error {
	return e.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.RemoveEventsByBlockID(rw.GlobalReader(), rw.Writer(), blockID)
	})
}

// BatchRemoveByBlockID removes events keyed by a blockID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (e *Events) BatchRemoveByBlockID(blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
	return operation.RemoveEventsByBlockID(rw.GlobalReader(), rw.Writer(), blockID)
}

type ServiceEvents struct {
	db    storage.DB
	cache *Cache[flow.Identifier, []flow.Event]
}

func NewServiceEvents(collector module.CacheMetrics, db storage.DB) *ServiceEvents {
	retrieve := func(r storage.Reader, blockID flow.Identifier) ([]flow.Event, error) {
		var events []flow.Event
		err := operation.LookupServiceEventsByBlockID(r, blockID, &events)
		return events, err
	}

	return &ServiceEvents{
		db: db,
		cache: newCache(collector, metrics.ResourceEvents,
			withStore(noopStore[flow.Identifier, []flow.Event]),
			withRetrieve(retrieve)),
	}
}

// BatchStore stores service events keyed by a blockID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (e *ServiceEvents) BatchStore(blockID flow.Identifier, events []flow.Event, rw storage.ReaderBatchWriter) error {
	writer := rw.Writer()
	for _, event := range events {
		err := operation.InsertServiceEvent(writer, blockID, event)
		if err != nil {
			return fmt.Errorf("cannot batch insert service event: %w", err)
		}
	}

	callback := func() {
		e.cache.Insert(blockID, events)
	}
	storage.OnCommitSucceed(rw, callback)
	return nil
}

// ByBlockID returns the events for the given block ID
func (e *ServiceEvents) ByBlockID(blockID flow.Identifier) ([]flow.Event, error) {
	val, err := e.cache.Get(e.db.Reader(), blockID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// RemoveByBlockID removes service events by block ID
func (e *ServiceEvents) RemoveByBlockID(blockID flow.Identifier) error {
	return e.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.RemoveServiceEventsByBlockID(rw.GlobalReader(), rw.Writer(), blockID)
	})
}

// BatchRemoveByBlockID removes service events keyed by a blockID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (e *ServiceEvents) BatchRemoveByBlockID(blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
	return e.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.RemoveServiceEventsByBlockID(rw.GlobalReader(), rw.Writer(), blockID)
	})
}
