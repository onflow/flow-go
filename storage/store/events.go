package store

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

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

	remove := func(rw storage.ReaderBatchWriter, blockID flow.Identifier) error {
		return operation.RemoveEventsByBlockID(rw.GlobalReader(), rw.Writer(), blockID)
	}

	return &Events{
		db: db,
		cache: newCache(collector, metrics.ResourceEvents,
			withStore(noopStore[flow.Identifier, []flow.Event]),
			withRetrieve(retrieve),
			withRemove[flow.Identifier, []flow.Event](remove),
		)}
}

// BatchStore will store events for the given block ID in a given batch
// It requires the caller to hold [storage.LockInsertEvent]
// Expected error returns:
//   - [storage.ErrAlreadyExists] if events for the block already exist.
func (e *Events) BatchStore(lctx lockctx.Proof, blockID flow.Identifier, blockEvents []flow.EventsList, batch storage.ReaderBatchWriter) error {
	err := operation.InsertBlockEvents(lctx, batch, blockID, blockEvents) // persists all events
	if err != nil {
		return fmt.Errorf("cannot batch insert events: %w", err)
	}

	// pre-allocating and indexing slice is faster than appending
	sliceSize := 0
	for _, b := range blockEvents {
		sliceSize += len(b)
	}

	combinedEvents := make([]flow.Event, sliceSize)
	eventIndex := 0

	for _, txEvents := range blockEvents {
		for _, event := range txEvents {
			combinedEvents[eventIndex] = event
			eventIndex++
		}
	}

	storage.OnCommitSucceed(batch, func() {
		e.cache.Insert(blockID, combinedEvents)
	})
	return nil
}

// ByBlockID returns the events for the given block ID.
// Note: This method will return an empty slice and no error if no entries for the blockID are found.
//
// The returned slice is a copy of the cached data. Callers may sort or otherwise
// mutate it without affecting subsequent calls or concurrent readers.
func (e *Events) ByBlockID(blockID flow.Identifier) ([]flow.Event, error) {
	val, err := e.cache.Get(e.db.Reader(), blockID)
	if err != nil {
		return nil, err
	}

	// each caller should get their own copy of the events to prevent mutation of the cached list.
	// this is important to prevent race conditions if callers use sort methods.
	result := make([]flow.Event, len(val))
	copy(result, val)

	// callers may also convert the payload from ccf to jsoncdc, so make an explicit copy of the payload
	// to avoid accidentally modifying the cached value.
	for i, event := range result {
		payload := make([]byte, len(event.Payload))
		copy(payload, event.Payload)
		result[i].Payload = payload
	}
	return result, nil
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
		return e.BatchRemoveByBlockID(blockID, rw)
	})
}

// BatchRemoveByBlockID removes events keyed by a blockID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
func (e *Events) BatchRemoveByBlockID(blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
	return e.cache.RemoveTx(rw, blockID)
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

	remove := func(rw storage.ReaderBatchWriter, blockID flow.Identifier) error {
		return operation.RemoveServiceEventsByBlockID(rw.GlobalReader(), rw.Writer(), blockID)
	}

	return &ServiceEvents{
		db: db,
		cache: newCache(collector, metrics.ResourceEvents,
			withStore(noopStore[flow.Identifier, []flow.Event]),
			withRetrieve(retrieve),
			withRemove[flow.Identifier, []flow.Event](remove),
		)}
}

// BatchStore stores service events keyed by the given blockID in as part of the write batch.
//
// Conceptually, this data should be written once for a block and never changed thereafter.
// This is enforced by the implementation, for which reason the caller must acquire
// [storage.LockInsertServiceEvent] and hold it until the batch is committed.
//
// Expected error returns:
//   - [storage.ErrAlreadyExists] if events for the block already exist.
func (e *ServiceEvents) BatchStore(lctx lockctx.Proof, blockID flow.Identifier, events []flow.Event, rw storage.ReaderBatchWriter) error {
	// Use the new InsertBlockServiceEvents operation to store all service events
	err := operation.InsertBlockServiceEvents(lctx, rw, blockID, events)
	if err != nil {
		return fmt.Errorf("cannot batch insert service events: %w", err)
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
func (e *ServiceEvents) BatchRemoveByBlockID(blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
	return e.cache.RemoveTx(rw, blockID)
}
