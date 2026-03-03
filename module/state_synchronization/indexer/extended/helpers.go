package extended

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// groupEventsByTxIndex returns a map of events grouped by transaction index in the original event order.
func groupEventsByTxIndex(events []flow.Event) map[uint32][]flow.Event {
	eventsByTxIndex := make(map[uint32][]flow.Event)
	for _, event := range events {
		eventsByTxIndex[event.TransactionIndex] = append(eventsByTxIndex[event.TransactionIndex], event)
	}
	return eventsByTxIndex
}

// flattenEvents converts a map of events grouped by transaction index into a flat slice.
// The order of events within each transaction group is preserved, but the order across groups is
// non-deterministic. Returns nil for nil or empty input.
func flattenEvents(eventsByTxIndex map[uint32][]flow.Event) []flow.Event {
	if len(eventsByTxIndex) == 0 {
		return nil
	}
	var events []flow.Event
	for _, txEvents := range eventsByTxIndex {
		events = append(events, txEvents...)
	}
	return events
}

// heightProvider is the common interface between FT and NFT bootstrapper stores needed to
// determine the next height to index.
type heightProvider interface {
	LatestIndexedHeight() (uint64, error)
	UninitializedFirstHeight() (uint64, bool)
}

// nextHeight computes the next height for a store that implements [heightProvider].
//
// No error returns are expected during normal operation.
func nextHeight(store heightProvider) (uint64, error) {
	height, err := store.LatestIndexedHeight()
	if err == nil {
		return height + 1, nil
	}

	if !errors.Is(err, storage.ErrNotBootstrapped) {
		return 0, fmt.Errorf("failed to get latest indexed height: %w", err)
	}

	firstHeight, isInitialized := store.UninitializedFirstHeight()
	if isInitialized {
		return 0, fmt.Errorf("failed to get latest indexed height, but index is initialized: %w", err)
	}

	return firstHeight, nil
}
