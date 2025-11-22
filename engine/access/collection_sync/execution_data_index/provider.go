package execution_data_index

import (
	"context"

	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/state_synchronization"
)

var _ collection_sync.ExecutionDataProvider = (*executionDataProvider)(nil)

// executionDataProvider implements ExecutionDataProvider by querying ExecutionDataCache.
type executionDataProvider struct {
	cache                      execution_data.ExecutionDataCache
	highestExectuionDataHeight state_synchronization.ExecutionDataIndexedHeight
}

// NewExecutionDataProvider creates a new ExecutionDataProvider that reads from the given ExecutionDataCache.
// The headers storage is used to determine the search range for finding available heights.
func NewExecutionDataProvider(
	cache execution_data.ExecutionDataCache,
	highestExectuionDataHeight state_synchronization.ExecutionDataIndexedHeight,
) *executionDataProvider {
	return &executionDataProvider{
		cache:                      cache,
		highestExectuionDataHeight: highestExectuionDataHeight,
	}
}

// HighestIndexedHeight returns the highest block height for which execution data is available.
func (p *executionDataProvider) HighestIndexedHeight() uint64 {
	return p.highestExectuionDataHeight.HighestConsecutiveHeight()
}

// GetExecutionDataByHeight returns the execution data for the given block height.
func (p *executionDataProvider) GetExecutionDataByHeight(ctx context.Context, height uint64) ([]*flow.Collection, error) {
	blockExecutionData, err := p.cache.ByHeight(ctx, height)
	if err != nil {
		return nil, err
	}

	// TODO: to support processing data from unsealed blocks, we should check here that the collections
	// returned match the guarantees in the block.

	return blockExecutionData.StandardCollections(), nil
}
