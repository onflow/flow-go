package storage

import "github.com/dapperlabs/flow-go/model/flow"

// TransactionErrors represents persistent storage for cadence errors that occurred during the execution of a transaction
type TransactionErrors interface {

	// Store will store events for the given block ID
	Store(blockID flow.Identifier, events []flow.Event) error

	// ByBlockID returns the events for the given block ID
	ByBlockID(blockID flow.Identifier) ([]flow.Event, error)

	// ByBlockIDTransactionID returns the events for the given block ID and transaction ID
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) ([]flow.Event, error)

	// ByBlockIDEventType returns the events for the given block ID and event type
	ByBlockIDEventType(blockID flow.Identifier, eventType flow.EventType) ([]flow.Event, error)
}
