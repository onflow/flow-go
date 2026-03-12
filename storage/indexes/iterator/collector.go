package iterator

import (
	"fmt"

	"github.com/onflow/flow-go/storage"
)

// CollectResults iterates over the storage iterator and collects results that match the filter.
// It returns when it reaches the limit or the iterator is exhausted.
// Returns the results matching the filter and the next cursor.
// A nil filter accepts all entries.
//
// No error returns are expected during normal operation.
func CollectResults[T any, C any](iter storage.IndexIterator[T, C], limit uint32, filter storage.IndexFilter[*T]) ([]T, *C, error) {
	if limit == 0 {
		return nil, nil, nil // no results to collect
	}

	var collected []T
	for item, err := range iter {
		if err != nil {
			return nil, nil, fmt.Errorf("could not get item: %w", err)
		}

		// stop once we've collected `limit` results
		// go one extra iteration to check if there are more results and build the next cursor
		// if there is no extra item, then the cursor will be nil
		if uint32(len(collected)) >= limit {
			nextItem := item
			nextCursor := nextItem.Cursor()
			return collected, &nextCursor, nil
		}

		tx, err := item.Value()
		if err != nil {
			return nil, nil, fmt.Errorf("could not get transaction: %w", err)
		}
		if filter != nil && !filter(&tx) {
			continue
		}
		collected = append(collected, tx)
	}
	return collected, nil, nil
}
