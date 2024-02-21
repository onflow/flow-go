package backend

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"
)

var _ state_synchronization.IndexReporter = (*EventsIndex)(nil)

type EventsIndex struct {
	events   storage.Events
	reporter *atomic.Pointer[state_synchronization.IndexReporter]
}

func NewEventsIndex(events storage.Events) *EventsIndex {
	return &EventsIndex{
		events:   events,
		reporter: atomic.NewPointer[state_synchronization.IndexReporter](nil),
	}
}

// Initialize replaces nil value with actual reporter instance
// Expected errors:
// - If the reporter was already initialized, return error
func (e *EventsIndex) Initialize(indexReporter state_synchronization.IndexReporter) error {
	if e.reporter.CompareAndSwap(nil, &indexReporter) {
		return nil
	}
	return fmt.Errorf("index reporter already initialized")
}

// ByBlockID checks data availability and returns events for a block
// Expected errors:
//   - indexer.ErrIndexNotInitialized: if the EventsIndex has not been initialized
//   - storage.ErrHeightNotIndexed: returned, when data is unavailable
//   - codes.NotFound: Result cannot be provided by storage due to the absence of data.
func (e *EventsIndex) ByBlockID(blockID flow.Identifier, height uint64) ([]flow.Event, error) {
	if err := e.checkDataAvailable(height); err != nil {
		return nil, err
	}

	return e.events.ByBlockID(blockID)
}

// ByBlockIDTransactionID checks data availability and return events for the given block ID and transaction ID
// Expected errors:
//   - indexer.ErrIndexNotInitialized: if the EventsIndex has not been initialized
//   - storage.ErrHeightNotIndexed: returned, when data is unavailable
//   - codes.NotFound: Result cannot be provided by storage due to the absence of data.
func (e *EventsIndex) ByBlockIDTransactionID(blockID flow.Identifier, height uint64, transactionID flow.Identifier) ([]flow.Event, error) {
	if err := e.checkDataAvailable(height); err != nil {
		return nil, err
	}

	return e.events.ByBlockIDTransactionID(blockID, transactionID)
}

// ByBlockIDTransactionIndex checks data availability and return events for the transaction at given index in a given block
// Expected errors:
//   - indexer.ErrIndexNotInitialized: if the EventsIndex has not been initialized
//   - storage.ErrHeightNotIndexed: returned, when data is unavailable
//   - codes.NotFound: Result cannot be provided by storage due to the absence of data.
func (e *EventsIndex) ByBlockIDTransactionIndex(blockID flow.Identifier, height uint64, txIndex uint32) ([]flow.Event, error) {
	if err := e.checkDataAvailable(height); err != nil {
		return nil, err
	}

	return e.events.ByBlockIDTransactionIndex(blockID, txIndex)
}

// LowestIndexedHeight returns the lowest height indexed by the execution state indexer.
// Expected errors:
// - indexer.ErrIndexNotInitialized: if the EventsIndex has not been initialized
func (e *EventsIndex) LowestIndexedHeight() (uint64, error) {
	reporter, err := e.getReporter()
	if err != nil {
		return 0, err
	}

	return reporter.LowestIndexedHeight()
}

// HighestIndexedHeight returns the highest height indexed by the execution state indexer.
// Expected errors:
// - indexer.ErrIndexNotInitialized: if the EventsIndex has not been initialized
func (e *EventsIndex) HighestIndexedHeight() (uint64, error) {
	reporter, err := e.getReporter()
	if err != nil {
		return 0, err
	}

	return reporter.HighestIndexedHeight()
}

func (e *EventsIndex) checkDataAvailable(height uint64) error {
	reporter, err := e.getReporter()
	if err != nil {
		return err
	}

	highestHeight, err := reporter.HighestIndexedHeight()
	if err != nil {
		return fmt.Errorf("could not get highest indexed height: %w", err)
	}
	if height > highestHeight {
		return fmt.Errorf("%w: block not indexed yet", storage.ErrHeightNotIndexed)
	}

	lowestHeight, err := reporter.LowestIndexedHeight()
	if err != nil {
		return fmt.Errorf("could not get lowest indexed height: %w", err)
	}
	if height < lowestHeight {
		return fmt.Errorf("%w: block is before lowest indexed height", storage.ErrHeightNotIndexed)
	}

	return nil
}

func (e *EventsIndex) getReporter() (state_synchronization.IndexReporter, error) {
	reporter := e.reporter.Load()
	if reporter == nil {
		return nil, indexer.ErrIndexNotInitialized
	}
	return *reporter, nil
}
