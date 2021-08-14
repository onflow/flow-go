package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type Events struct {
	db    *badger.DB
	cache *Cache
}

func NewEvents(collector module.CacheMetrics, db *badger.DB) *Events {
	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		blockID := key.(flow.Identifier)
		var events []flow.Event
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.LookupEventsByBlockID(blockID, &events)(tx)
			return events, handleError(err, flow.Event{})
		}
	}

	return &Events{
		db: db,
		cache: newCache(collector, metrics.ResourceEvents,
			withStore(noopStore),
			withRetrieve(retrieve)),
	}
}

func (e *Events) BatchStore(blockID flow.Identifier, blockEvents []flow.EventsList, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()

	// pre-allocating and indexing slice is faster than appending
	sliceSize := 0
	for _, b := range blockEvents {
		sliceSize += len(b)
	}

	combinedEvents := make([]flow.Event, sliceSize)

	eventIndex := 0

	for _, events := range blockEvents {
		for _, event := range events {
			err := operation.BatchInsertEvent(blockID, event)(writeBatch)
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
	batch.OnSucceed(callback)
	return nil
}

// ByBlockID returns the events for the given block ID
func (e *Events) ByBlockID(blockID flow.Identifier) ([]flow.Event, error) {
	tx := e.db.NewTransaction(false)
	defer tx.Discard()
	val, err := e.cache.Get(blockID)(tx)
	if err != nil {
		return nil, err
	}
	return val.([]flow.Event), nil
}

// ByBlockIDTransactionID returns the events for the given block ID and transaction ID
func (e *Events) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) ([]flow.Event, error) {
	events, err := e.ByBlockID(blockID)
	if err != nil {
		return nil, handleError(err, flow.Event{})
	}

	var matched []flow.Event
	for _, event := range events {
		if event.TransactionID == txID {
			matched = append(matched, event)
		}
	}
	return matched, nil
}

// ByBlockIDEventType returns the events for the given block ID and event type
func (e *Events) ByBlockIDEventType(blockID flow.Identifier, eventType flow.EventType) ([]flow.Event, error) {
	events, err := e.ByBlockID(blockID)
	if err != nil {
		return nil, handleError(err, flow.Event{})
	}

	var matched []flow.Event
	for _, event := range events {
		if event.Type == eventType {
			matched = append(matched, event)
		}
	}
	return matched, nil
}

type ServiceEvents struct {
	db    *badger.DB
	cache *Cache
}

func NewServiceEvents(collector module.CacheMetrics, db *badger.DB) *ServiceEvents {
	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		blockID := key.(flow.Identifier)
		var events []flow.Event
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.LookupServiceEventsByBlockID(blockID, &events)(tx)
			return events, handleError(err, flow.Event{})
		}
	}

	return &ServiceEvents{
		db: db,
		cache: newCache(collector, metrics.ResourceEvents,
			withStore(noopStore),
			withRetrieve(retrieve)),
	}
}

func (e *ServiceEvents) BatchStore(blockID flow.Identifier, events []flow.Event, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()
	for _, event := range events {
		err := operation.BatchInsertServiceEvent(blockID, event)(writeBatch)
		if err != nil {
			return fmt.Errorf("cannot batch insert service event: %w", err)
		}
	}

	callback := func() {
		e.cache.Insert(blockID, events)
	}
	batch.OnSucceed(callback)
	return nil
}

// ByBlockID returns the events for the given block ID
func (e *ServiceEvents) ByBlockID(blockID flow.Identifier) ([]flow.Event, error) {
	tx := e.db.NewTransaction(false)
	defer tx.Discard()
	val, err := e.cache.Get(blockID)(tx)
	if err != nil {
		return nil, err
	}
	return val.([]flow.Event), nil
}
