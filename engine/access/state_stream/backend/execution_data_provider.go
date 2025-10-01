package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/storage"
)

type ExecutionDataProvider interface {
	ExecutionData(ctx context.Context, height uint64) (*execution_data.BlockExecutionDataEntity, error)
}

type ExecutionDataProviderImpl struct {
	execDataTracker tracker.ExecutionDataTracker
	execDataCache   *cache.ExecutionDataCache
}

var _ ExecutionDataProvider = (*ExecutionDataProviderImpl)(nil)

func NewExecutionDataProvider(
	execDataTracker tracker.ExecutionDataTracker,
	execDataCache *cache.ExecutionDataCache,
) *ExecutionDataProviderImpl {
	return &ExecutionDataProviderImpl{
		execDataTracker: execDataTracker,
		execDataCache:   execDataCache,
	}
}

func (e *ExecutionDataProviderImpl) ExecutionData(
	ctx context.Context,
	height uint64,
) (*execution_data.BlockExecutionDataEntity, error) {
	highestHeight := e.execDataTracker.GetHighestHeight()
	// fail early if no notification has been received for the given block height.
	// note: it's possible for the data to exist in the data store before the notification is
	// received. this ensures a consistent view is available to all streams.
	if height > highestHeight {
		return nil, fmt.Errorf("execution data for block %d is not available yet: %w", height, subscription.ErrBlockNotReady)
	}

	execData, err := e.execDataCache.ByHeight(ctx, height)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) ||
			execution_data.IsBlobNotFoundError(err) {
			err = errors.Join(err, subscription.ErrBlockNotReady)
			return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
		}
		return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
	}

	return execData, nil
}
