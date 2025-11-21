package factory

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/engine/access/collection_sync/execution_data_index"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/storage"
)

// CreateExecutionDataProcessor creates a new ExecutionDataProcessor with the provided dependencies.
//
// Parameters:
//   - log: Logger for the component
//   - cache: Execution data cache for retrieving execution data by height
//   - executionDataTracker: Tracker for execution data that provides the highest available height
//   - processedHeight: Consumer progress for tracking processed heights
//   - indexer: Block collection indexer for indexing collections
//   - collectionSyncMetrics: Optional metrics collector for tracking collection sync progress
//
// Returns:
//   - *ExecutionDataProcessor: A new ExecutionDataProcessor instance
//   - error: An error if the processor could not be created
//
// No errors are expected during normal operation.
func CreateExecutionDataProcessor(
	log zerolog.Logger,
	cache execution_data.ExecutionDataCache,
	indexedHeight state_synchronization.ExecutionDataIndexedHeight,
	processedHeight storage.ConsumerProgress,
	indexer collection_sync.BlockCollectionIndexer,
	collectionSyncMetrics module.CollectionSyncMetrics, // optional metrics collector
) (*execution_data_index.ExecutionDataProcessor, error) {
	// Create execution data provider
	executionDataProvider := execution_data_index.NewExecutionDataProvider(cache, indexedHeight)

	// Convert ConsumerProgress to PersistentStrictMonotonicCounter
	processedHeightCounter, err := counters.NewPersistentStrictMonotonicCounter(processedHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistent strict monotonic counter: %w", err)
	}

	// Create the execution data processor
	processor := execution_data_index.NewExecutionDataProcessor(log, executionDataProvider, indexer, processedHeightCounter, collectionSyncMetrics)

	return processor, nil
}
