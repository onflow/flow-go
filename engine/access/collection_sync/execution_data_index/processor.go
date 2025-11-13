package execution_data_index

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type ExecutionDataProcessor struct {
	component.Component
	newExecutionDataIndexed chan struct{}
	provider                collection_sync.ExecutionDataProvider
	indexer                 collection_sync.BlockCollectionIndexer
	// state
	processedHeight *counters.PersistentStrictMonotonicCounter
	metrics         module.CollectionSyncMetrics
}

var _ collection_sync.ExecutionDataProcessor = (*ExecutionDataProcessor)(nil)
var _ collection_sync.ProgressReader = (*ExecutionDataProcessor)(nil)
var _ component.Component = (*ExecutionDataProcessor)(nil)

func NewExecutionDataProcessor(
	provider collection_sync.ExecutionDataProvider,
	indexer collection_sync.BlockCollectionIndexer,
	processedHeight *counters.PersistentStrictMonotonicCounter,
	metrics module.CollectionSyncMetrics, // optional metrics collector
) *ExecutionDataProcessor {
	edp := &ExecutionDataProcessor{
		newExecutionDataIndexed: make(chan struct{}, 1),
		provider:                provider,
		indexer:                 indexer,
		processedHeight:         processedHeight,
		metrics:                 metrics,
	}

	// Initialize the channel so that even if no new execution data comes in,
	// the worker loop can still be triggered to process any existing data.
	edp.newExecutionDataIndexed <- struct{}{}

	// Build component manager with worker loop
	cm := component.NewComponentManagerBuilder().
		AddWorker(edp.workerLoop).
		Build()

	edp.Component = cm

	return edp
}

func (edp *ExecutionDataProcessor) OnNewExectuionData() {
	select {
	case edp.newExecutionDataIndexed <- struct{}{}:
	default:
		// if the channel is full, no need to block, just return.
		// once the worker loop processes the buffered signal, it will
		// process the next height all the way to the highest available height.
	}
}

func (edp *ExecutionDataProcessor) workerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	// using a single threaded loop to index each execution for height
	// since indexing collections is blocking anyway, and reading the execution data
	// is quick, because we cache for 100 heights.
	for {
		select {
		case <-ctx.Done():
			return
		case <-edp.newExecutionDataIndexed:
			highestAvailableHeight := edp.provider.HighestIndexedHeight()
			lowestMissing := edp.processedHeight.Value() + 1

			for height := lowestMissing; height <= highestAvailableHeight; height++ {
				collections, err := edp.provider.GetExecutionDataByHeight(ctx, height)
				if err != nil {
					ctx.Throw(fmt.Errorf("failed to get execution data for height %d: %w", height, err))
					return
				}

				// TODO: since both collections and execution data processor are the data source of
				// collections, before indexing the collections, double check if it was indexed
				// by the collections already by simply comparing the missing height with the
				// collections's lowest height.
				// if collections's lowest height is higher than the missing height, it means the collections
				// has been indexed by the collections already, no need to index again.
				// And make sure reading the collections's lowest height is cheap operation (only hitting RW lock)

				err = edp.indexer.IndexCollectionsForBlock(height, collections)
				// TODO: handle already exists
				if err != nil {
					ctx.Throw(fmt.Errorf("failed to index collections for block height %d: %w", height, err))
					return
				}

				// Update processed height after successful indexing
				err = edp.processedHeight.Set(height)
				if err != nil {
					ctx.Throw(fmt.Errorf("failed to update processed height to %d: %w", height, err))
					return
				}

				// Update metrics if available
				if edp.metrics != nil {
					edp.metrics.CollectionSyncedHeight(height)
				}
			}
		}
	}
}

func (edp *ExecutionDataProcessor) ProcessedHeight() uint64 {
	return edp.processedHeight.Value()
}
