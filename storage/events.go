package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Events represents persistent storage for events.
type Events interface {
	// Store will store events for the given block ID
	Store(blockID flow.Identifier, blockEvents []flow.EventsList) error

	// BatchStore will store events for the given block ID in a given batch
	BatchStore(blockID flow.Identifier, events []flow.EventsList, batch BatchStorage) error

	// ByBlockID returns the events for the given block ID
	ByBlockID(blockID flow.Identifier) ([]flow.Event, error)

	// ByBlockIDTransactionID returns the events for the given block ID and transaction ID
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) ([]flow.Event, error)

	// ByBlockIDTransactionIndex returns the events for the transaction at given index in a given block
	ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) ([]flow.Event, error)

	// ByBlockIDEventType returns the events for the given block ID and event type
	ByBlockIDEventType(blockID flow.Identifier, eventType flow.EventType) ([]flow.Event, error)

	// BatchRemoveByBlockID removes events keyed by a blockID in provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemoveByBlockID(blockID flow.Identifier, batch BatchStorage) error
}

type ServiceEvents interface {
	// BatchStore stores service events keyed by a blockID in provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchStore(blockID flow.Identifier, events []flow.Event, batch BatchStorage) error

	// ByBlockID returns the events for the given block ID
	ByBlockID(blockID flow.Identifier) ([]flow.Event, error)

	// BatchRemoveByBlockID removes service events keyed by a blockID in provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemoveByBlockID(blockID flow.Identifier, batch BatchStorage) error
}
