package stores

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
)

var _ PersisterStore = (*EventsStore)(nil)

// EventsStore handles persisting events
type EventsStore struct {
	inMemoryEvents  *unsynchronized.Events
	persistedEvents storage.Events
	blockID         flow.Identifier
}

func NewEventsStore(
	inMemoryEvents *unsynchronized.Events,
	persistedEvents storage.Events,
	blockID flow.Identifier,
) *EventsStore {
	return &EventsStore{
		inMemoryEvents:  inMemoryEvents,
		persistedEvents: persistedEvents,
		blockID:         blockID,
	}
}

// Persist adds events to the batch.
// No errors are expected during normal operations
func (e *EventsStore) Persist(lctx lockctx.Proof, batch storage.ReaderBatchWriter) error {
	eventsList, err := e.inMemoryEvents.ByBlockID(e.blockID)
	if err != nil {
		return fmt.Errorf("could not get events: %w", err)
	}

	if len(eventsList) > 0 {
		if err := e.persistedEvents.BatchStore(e.blockID, []flow.EventsList{eventsList}, batch); err != nil {
			return fmt.Errorf("could not add events to batch: %w", err)
		}
	}

	return nil
}
