package factory

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/engine/access/collection_sync/execution_data_index"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/storage"
)

// createExecutionDataProcessor creates a new ExecutionDataProcessor with the provided dependencies.
//
// Parameters:
//   - log: Logger for the component
//   - cache: Execution data cache for retrieving execution data by height
//   - executionDataTracker: Tracker for execution data that provides the highest available height
//   - processedHeight: Consumer progress for tracking processed heights
//   - indexer: Block collection indexer for indexing collections
//   - onIndexedCallback: Callback function to be called when a block's execution data has been indexed
//
// Returns:
//   - *ExecutionDataProcessor: A new ExecutionDataProcessor instance
//   - error: An error if the processor could not be created
//
// No errors are expected during normal operation.
func createExecutionDataProcessor(
	log zerolog.Logger,
	executionDataProvider collection_sync.ExecutionDataProvider,
	processedHeight storage.ConsumerProgress,
	indexer collection_sync.BlockCollectionIndexer,
	onIndexedCallback func(uint64),
) (*execution_data_index.ExecutionDataProcessor, error) {
	// Convert ConsumerProgress to PersistentStrictMonotonicCounter
	processedHeightCounter, err := counters.NewPersistentStrictMonotonicCounter(processedHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistent strict monotonic counter: %w", err)
	}

	// Create the execution data processor
	processor := execution_data_index.NewExecutionDataProcessor(log, executionDataProvider, indexer, processedHeightCounter, onIndexedCallback)

	return processor, nil
}
