package collections

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

// CreateExecutionDataProcessor creates a new ExecutionDataProcessor with the provided dependencies.
//
// Parameters:
//   - cache: Execution data cache for retrieving execution data by height
//   - executionDataTracker: Tracker for execution data that provides the highest available height
//   - processedHeight: Consumer progress for tracking processed heights
//   - indexer: Block collection indexer for indexing collections
//
// Returns:
//   - *ExecutionDataProcessor: A new ExecutionDataProcessor instance
//   - error: An error if the processor could not be created
//
// No errors are expected during normal operation.
func CreateExecutionDataProcessor(
	cache execution_data.ExecutionDataCache,
	executionDataTracker tracker.ExecutionDataTracker,
	processedHeight storage.ConsumerProgress,
	indexer collection_sync.BlockCollectionIndexer,
) (*ExecutionDataProcessor, error) {
	// Create EDI height provider
	ediHeightProvider := NewEDIHeightProvider(cache, executionDataTracker)

	// Convert ConsumerProgress to PersistentStrictMonotonicCounter
	processedHeightCounter, err := counters.NewPersistentStrictMonotonicCounter(processedHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistent strict monotonic counter: %w", err)
	}

	// Create the execution data processor
	processor := NewExecutionDataProcessor(ediHeightProvider, indexer, processedHeightCounter)

	return processor, nil
}
