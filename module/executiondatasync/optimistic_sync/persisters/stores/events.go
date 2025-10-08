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
//
// No error returns are expected during normal operations
func (e *EventsStore) Persist(_ lockctx.Proof, batch storage.ReaderBatchWriter) error {
	err := storage.SkipAlreadyExistsError( // Note: if the data already exists, we will not overwrite
		e.persistedEvents.BatchStore(e.blockID, []flow.EventsList{e.data}, batch),
	)

	if err != nil {
		return fmt.Errorf("could not add events to batch: %w", err)
	}

	return nil
}
