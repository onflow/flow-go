package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Events struct {
	db *badger.DB
}

func NewEvents(db *badger.DB) *Events {
	return &Events{
		db: db,
	}
}

// Store will store events for the given block ID
func (e *Events) Store(blockID flow.Identifier, events []flow.Event) error {
	return operation.RetryOnConflict(e.db.Update, func(btx *badger.Txn) error {
		for _, event := range events {
			err := operation.InsertEvent(blockID, event)(btx)
			if err != nil {
				return fmt.Errorf("could not insert event: %w", err)
			}
		}
		return nil
	})
}

// ByBlockID returns the events for the given block ID
func (e *Events) ByBlockID(blockID flow.Identifier) ([]flow.Event, error) {

	var events []flow.Event
	err := e.db.View(operation.LookupEventsByBlockID(blockID, &events))
	if err != nil {
		return nil, handleError(err, flow.Event{})
	}

	return events, nil
}

// ByBlockIDTransactionID returns the events for the given block ID and transaction ID
func (e *Events) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) ([]flow.Event, error) {

	var events []flow.Event
	err := e.db.View(operation.RetrieveEvents(blockID, txID, &events))
	if err != nil {
		return nil, handleError(err, flow.Event{})
	}

	return events, nil
}

// ByBlockIDEventType returns the events for the given block ID and event type
func (e *Events) ByBlockIDEventType(blockID flow.Identifier, event flow.EventType) ([]flow.Event, error) {

	var events []flow.Event
	err := e.db.View(operation.LookupEventsByBlockIDEventType(blockID, event, &events))
	if err != nil {
		return nil, handleError(err, flow.Event{})
	}

	return events, nil
}
