package stores

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ PersisterStore = (*EventsStore)(nil)

// EventsStore handles persisting events
type EventsStore struct {
	data    []flow.Event
	events  storage.Events
	blockID flow.Identifier
}

func NewEventsStore(
	data []flow.Event,
	events storage.Events,
	blockID flow.Identifier,
) *EventsStore {
	return &EventsStore{
		data:    data,
		events:  events,
		blockID: blockID,
	}
}

// Persist adds events to the batch.
// The caller must acquire [storage.LockInsertEvent] and hold it until the write batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if events for the block already exist.
func (e *EventsStore) Persist(lctx lockctx.Proof, batch storage.ReaderBatchWriter) error {
	err := e.events.BatchStore(lctx, e.blockID, []flow.EventsList{e.data}, batch)
	if err != nil {
		return fmt.Errorf("could not add events to batch: %w", err)
	}
	return nil
}
