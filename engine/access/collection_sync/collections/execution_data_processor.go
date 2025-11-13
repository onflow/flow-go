package collections

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type ExecutionDataProcessor struct {
	component.Component
	newExecutionDataIndexed chan struct{}
	ediHeightProvider       collection_sync.EDIHeightProvider
	indexer                 collection_sync.BlockCollectionIndexer
	// state
	processedHeight *counters.PersistentStrictMonotonicCounter
}

var _ collection_sync.ExecutionDataProcessor = (*ExecutionDataProcessor)(nil)
var _ component.Component = (*ExecutionDataProcessor)(nil)

func NewExecutionDataProcessor(
	ediHeightProvider collection_sync.EDIHeightProvider,
	indexer collection_sync.BlockCollectionIndexer,
	processedHeight *counters.PersistentStrictMonotonicCounter,
) *ExecutionDataProcessor {
	edp := &ExecutionDataProcessor{
		newExecutionDataIndexed: make(chan struct{}, 1),
		ediHeightProvider:       ediHeightProvider,
		indexer:                 indexer,
		processedHeight:         processedHeight,
	}

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
			highestAvailableHeight := edp.ediHeightProvider.HighestIndexedHeight()
			lowestMissing := edp.processedHeight.Value() + 1

			for height := lowestMissing; height <= highestAvailableHeight; height++ {
				collections, err := edp.ediHeightProvider.GetExecutionDataByHeight(ctx, height)
				if err != nil {
					ctx.Throw(fmt.Errorf("failed to get execution data for height %d: %w", height, err))
					return
				}

				// TODO: since both fetcher and execution data processor are the data source of
				// collections, before indexing the collections, double check if it was indexed
				// by the fetcher already by simply comparing the missing height with the
				// fetcher's lowest height.
				// if fetcher's lowest height is higher than the missing height, it means the collections
				// has been indexed by the fetcher already, no need to index again.
				// And make sure reading the fetcher's lowest height is cheap operation (only hitting RW lock)

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
			}
		}
	}
}
