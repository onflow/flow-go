package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

type EventsReader interface {
	// ByBlockID returns the events for the given block ID
	ByBlockID(blockID flow.Identifier) ([]flow.Event, error)

	// ByBlockIDTransactionID returns the events for the given block ID and transaction ID
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) ([]flow.Event, error)

	// ByBlockIDTransactionIndex returns the events for the transaction at given index in a given block
	ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) ([]flow.Event, error)

	// ByBlockIDEventType returns the events for the given block ID and event type
	ByBlockIDEventType(blockID flow.Identifier, eventType flow.EventType) ([]flow.Event, error)
}

// Events represents persistent storage for events.
type Events interface {
	EventsReader

	// BatchStore will store events for the given block ID in a given batch
	// it requires the caller to hold [storage.LockInsertEvent]
	// It returns [storage.ErrAlreadyExists] if events for the block already exist.
	BatchStore(lctx lockctx.Proof, blockID flow.Identifier, events []flow.EventsList, batch ReaderBatchWriter) error

	// BatchRemoveByBlockID removes events keyed by a blockID in provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If database unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemoveByBlockID(blockID flow.Identifier, batch ReaderBatchWriter) error
}

type ServiceEvents interface {
	// BatchStore stores service events keyed by a blockID in provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// it requires the caller to hold [storage.LockInsertServiceEvent]
	// It returns [storage.ErrAlreadyExists] if any service events for the block already exist.
	BatchStore(lctx lockctx.Proof, blockID flow.Identifier, events []flow.Event, batch ReaderBatchWriter) error

	// ByBlockID returns the events for the given block ID
	ByBlockID(blockID flow.Identifier) ([]flow.Event, error)

	// BatchRemoveByBlockID removes service events keyed by a blockID in provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If database unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemoveByBlockID(blockID flow.Identifier, batch ReaderBatchWriter) error
}
