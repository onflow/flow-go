package stores

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ PersisterStore = (*EventsStore)(nil)

// EventsStore handles persisting events
type EventsStore struct {
	data            []flow.Event
	persistedEvents storage.Events
	blockID         flow.Identifier
}

func NewEventsStore(
	data []flow.Event,
	persistedEvents storage.Events,
	blockID flow.Identifier,
) *EventsStore {
	return &EventsStore{
		data:            data,
		persistedEvents: persistedEvents,
		blockID:         blockID,
	}
}

// Persist adds events to the batch.
// The caller must acquire [storage.LockInsertEvent] and hold it until the write batch is  committed.
//
// No error returns are expected during normal operations
func (e *EventsStore) Persist(lctx lockctx.Proof, batch storage.ReaderBatchWriter) error {
	err := e.persistedEvents.BatchStore(lctx, e.blockID, []flow.EventsList{e.data}, batch)
	if err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			// CAUTION: here we assume that if something is already stored for our blockID, then the data is identical.
			// This only holds true for sealed execution results, whose consistency has previously been verified by
			// comparing the data's hash to commitments in the execution result.
			return nil
		}
		return fmt.Errorf("could not add events to batch: %w", err)
	}

	return nil
}
