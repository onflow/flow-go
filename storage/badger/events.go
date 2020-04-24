package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
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
	return e.db.Update(func(btx *badger.Txn) error {
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
	err := e.db.View(func(btx *badger.Txn) error {
		err := operation.LookupEventsByBlockID(blockID, &events)(btx)
		return handleError(err, flow.Event{})
	})

	if err != nil {
		return nil, err
	}

	return events, nil
}

// ByBlockIDTransactionID returns the events for the given block ID and transaction ID
func (e *Events) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) ([]flow.Event, error) {

	var events *[]flow.Event
	err := e.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveEvents(blockID, txID, events)(btx)
		return handleError(err, flow.Event{})
	})

	if err != nil {
		return nil, err
	}

	return *events, nil
}

// ByBlockIDEventType returns the events for the given block ID and event type
func (e *Events) ByBlockIDEventType(blockID flow.Identifier, event flow.EventType) ([]flow.Event, error) {

	var events *[]flow.Event
	err := e.db.View(func(btx *badger.Txn) error {
		err := operation.LookupEventsByBlockIDEventType(blockID, event, events)(btx)
		return handleError(err, flow.Event{})
	})

	if err != nil {
		return nil, err
	}

	return *events, nil
}

func handleError(err error, t interface{}) error {
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("could not retrieve %T: %w", t, storage.ErrNotFound)
		}
		return fmt.Errorf("could not retrieve %T: %w", t, err)
	}
	return nil
}
