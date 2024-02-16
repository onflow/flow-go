package backend

import (
	"fmt"
	"sort"

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

func (e *EventsIndex) Initialize(indexReporter state_synchronization.IndexReporter) error {
	if e.reporter.CompareAndSwap(nil, &indexReporter) {
		return nil
	}
	return fmt.Errorf("index reporter already initialized")
}

func (e *EventsIndex) GetEvents(blockID flow.Identifier, height uint64) ([]flow.Event, error) {
	if err := e.checkDataAvailable(height); err != nil {
		return nil, err
	}

	events, err := e.events.ByBlockID(blockID)
	if err != nil {
		return nil, err
	}

	// events are keyed/sorted by [blockID, txID, txIndex, eventIndex]
	// we need to resort them by tx index then event index so the output is in execution order
	sort.Slice(events, func(i, j int) bool {
		if events[i].TransactionIndex == events[j].TransactionIndex {
			return events[i].EventIndex < events[j].EventIndex
		}
		return events[i].TransactionIndex < events[j].TransactionIndex
	})

	return events, nil
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
