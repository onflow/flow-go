package collections

import (
	"context"

	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

var _ collection_sync.EDIHeightProvider = (*ediHeightProvider)(nil)

// ediHeightProvider implements EDIHeightProvider by querying ExecutionDataCache.
type ediHeightProvider struct {
	cache                      execution_data.ExecutionDataCache
	highestExectuionDataHeight tracker.ExecutionDataTracker
}

// NewEDIHeightProvider creates a new EDIHeightProvider that reads from the given ExecutionDataCache.
// The headers storage is used to determine the search range for finding available heights.
func NewEDIHeightProvider(
	cache execution_data.ExecutionDataCache,
	highestExectuionDataHeight tracker.ExecutionDataTracker,
) *ediHeightProvider {
	return &ediHeightProvider{
		cache:                      cache,
		highestExectuionDataHeight: highestExectuionDataHeight,
	}
}

// HighestIndexedHeight returns the highest block height for which execution data is available.
func (p *ediHeightProvider) HighestIndexedHeight() uint64 {
	return p.highestExectuionDataHeight.GetHighestHeight()
}

// GetExecutionDataByHeight returns the execution data for the given block height.
func (p *ediHeightProvider) GetExecutionDataByHeight(ctx context.Context, height uint64) ([]*flow.Collection, error) {
	blockExecutionData, err := p.cache.ByHeight(ctx, height)
	if err != nil {
		return nil, err
	}

	return blockExecutionData.StandardCollections(), nil
}
