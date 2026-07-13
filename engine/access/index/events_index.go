package index

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// EventsIndex implements a wrapper around `storage.Events` ensuring that needed data has been synced and is available to the client.
// Note: read detail how `Reporter` is working
type EventsIndex struct {
	*Reporter
	events storage.Events
}

func NewEventsIndex(reporter *Reporter, events storage.Events) *EventsIndex {
	return &EventsIndex{
		Reporter: reporter,
		events:   events,
	}
}

// ByBlockID checks data availability and returns events for a block
// Expected errors:
//   - indexer.ErrIndexNotInitialized if the `EventsIndex` has not been initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
func (e *EventsIndex) ByBlockID(blockID flow.Identifier, height uint64) ([]flow.Event, error) {
	if err := e.checkDataAvailability(height); err != nil {
		return nil, err
	}

	events, err := e.events.ByBlockID(blockID)
	if err != nil {
		return nil, err
	}

	return events, nil
}

// ByBlockIDTransactionID checks data availability and return events for the given block ID and transaction ID
// Expected errors:
//   - indexer.ErrIndexNotInitialized if the `EventsIndex` has not been initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
func (e *EventsIndex) ByBlockIDTransactionID(blockID flow.Identifier, height uint64, transactionID flow.Identifier) ([]flow.Event, error) {
	if err := e.checkDataAvailability(height); err != nil {
		return nil, err
	}

	return e.events.ByBlockIDTransactionID(blockID, transactionID)
}

// ByBlockIDTransactionIndex checks data availability and return events for the transaction at given index in a given block
// Expected errors:
//   - indexer.ErrIndexNotInitialized if the `EventsIndex` has not been initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
func (e *EventsIndex) ByBlockIDTransactionIndex(blockID flow.Identifier, height uint64, txIndex uint32) ([]flow.Event, error) {
	if err := e.checkDataAvailability(height); err != nil {
		return nil, err
	}

	return e.events.ByBlockIDTransactionIndex(blockID, txIndex)
}
