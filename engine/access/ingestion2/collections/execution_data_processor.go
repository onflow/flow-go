package collections

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/ingestion2"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type ExecutionDataProcessor struct {
	newExecutionDataIndexed chan struct{}
	ediHeightProvider       ingestion2.EDIHeightProvider
	indexer                 ingestion2.BlockCollectionIndexer
	// state
	processedHeight *counters.PersistentStrictMonotonicCounter
}

var _ ingestion2.ExecutionDataProcessor = (*ExecutionDataProcessor)(nil)

func NewExecutionDataProcessor(
	ediHeightProvider ingestion2.EDIHeightProvider,
	processedHeight *counters.PersistentStrictMonotonicCounter,
) *ExecutionDataProcessor {
	return &ExecutionDataProcessor{
		newExecutionDataIndexed: make(chan struct{}, 1),
		ediHeightProvider:       ediHeightProvider,
		processedHeight:         processedHeight,
	}
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

func (edp *ExecutionDataProcessor) WrokerLoop(ctx irrecoverable.SignalerContext) error {
	// using a single threaded loop to index each exectuion for height
	// since indexing collections is blocking anyway, and reading the execution data
	// is quick, because we cache for 100 heights.
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-edp.newExecutionDataIndexed:
			highestAvailableHeight := edp.ediHeightProvider.HighestIndexedHeight()
			lowestMissing := edp.processedHeight.Value() + 1

			for height := lowestMissing; height <= highestAvailableHeight; height++ {
				collections, err := edp.ediHeightProvider.GetExecutionDataByHeight(height)
				if err != nil {
					return fmt.Errorf("failed to get execution data for height %d: %w", height, err)
				}

				// TODO: since both fetcher and exectuion data processor are the data source of
				// collections, before indexing the collections, double check if it was indexed
				// by the fetcher already by simply comparing the missing height with the
				// fetcher's lowest height.
				// if fetcher's lowest height is higher than the missing height, it means the collections
				// has been indexed by the fetcher already, no need to index again.
				// And make sure reading the fetcher's lowest height is cheap operation (only hitting RW lock)

				err = edp.indexer.IndexCollectionsForBlock(height, collections)
				if err != nil {
					return fmt.Errorf("failed to index collections for block height %d: %w", height, err)
				}
			}
		}
	}
}
