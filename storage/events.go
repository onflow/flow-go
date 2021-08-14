package storage

import "github.com/onflow/flow-go/model/flow"

// Events represents persistent storage for events.
type Events interface {

	// BatchStore will store events for the given block ID in a given batch
	BatchStore(blockID flow.Identifier, events []flow.EventsList, batch BatchStorage) error

	// ByBlockID returns the events for the given block ID
	ByBlockID(blockID flow.Identifier) ([]flow.Event, error)

	// ByBlockIDTransactionID returns the events for the given block ID and transaction ID
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) ([]flow.Event, error)

	// ByBlockIDEventType returns the events for the given block ID and event type
	ByBlockIDEventType(blockID flow.Identifier, eventType flow.EventType) ([]flow.Event, error)
}

type ServiceEvents interface {
	// BatchStore will store service events for the given block ID in a given batch
	BatchStore(blockID flow.Identifier, events []flow.Event, batch BatchStorage) error

	// ByBlockID returns the events for the given block ID
	ByBlockID(blockID flow.Identifier) ([]flow.Event, error)
}
