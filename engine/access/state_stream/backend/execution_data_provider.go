package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// executionDataProvider connects subscription/streamer with backends.
// It is intended to be used as a data provider for the subscription package.
type executionDataProvider struct {
	state                   protocol.State
	headers                 storage.Headers
	executionDataTracker    tracker.ExecutionDataTracker
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider
	executionStateCache     optimistic_sync.ExecutionStateCache
	criteria                optimistic_sync.Criteria
	height                  uint64
}

func newExecutionDataProvider(
	state protocol.State,
	headers storage.Headers,
	executionDataTracker tracker.ExecutionDataTracker,
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider,
	executionStateCache optimistic_sync.ExecutionStateCache,
	nextCriteria optimistic_sync.Criteria,
	startHeight uint64,
) *executionDataProvider {
	return &executionDataProvider{
		state:                   state,
		headers:                 headers,
		executionDataTracker:    executionDataTracker,
		executionResultProvider: executionResultProvider,
		executionStateCache:     executionStateCache,
		criteria:                nextCriteria,
		height:                  startHeight,
	}
}

var _ subscription.DataProvider = (*executionDataProvider)(nil)

// NextData returns the execution data for the next block height.
// It is intended to be used by the streamer to fetch data sequentially.
//
//   - [subscription.ErrBlockNotReady]: If the execution data is not yet available. This includes cases where
//     the block is not finalized yet, or the execution result is pending (e.g. not enough agreeing executors).
//   - [optimistic_sync.ErrBlockBeforeNodeHistory]: If the request is for data before the node's root block.
func (e *executionDataProvider) NextData(ctx context.Context) (any, error) {
	availableFinalizedHeight := e.executionDataTracker.GetHighestAvailableFinalizedHeight()
	if e.height > availableFinalizedHeight {
		// fail early if no notification has been received for the given block height.
		// note: it's possible for the data to exist in the data store before the notification is
		// received. this ensures a consistent view is available to all streams.
		return nil, subscription.ErrBlockNotReady
	}

	// the spork root block will never have execution data available. If requested, return an empty result.
	if e.height == e.state.Params().SporkRootBlockHeight() {
		response := &ExecutionDataResponse{
			Height: e.height,
			ExecutionData: &execution_data.BlockExecutionData{
				BlockID: e.state.Params().SporkRootBlock().ID(),
			},
		}

		e.height += 1
		return response, nil
	}

	blockID, err := e.headers.BlockIDByHeight(e.height)
	if err != nil {
		// this function is called after the headers are updated, so if we didn't find the block header in the storage,
		// we treat it as an exception
		return nil, fmt.Errorf("block %d might not be finalized yet: %w", e.height, err)
	}

	execResultInfo, err := e.executionResultProvider.ExecutionResultInfo(blockID, e.criteria)
	if err != nil {
		switch {
		case optimistic_sync.IsExecutionResultNotReadyError(err):
			return nil, errors.Join(subscription.ErrBlockNotReady, err)

		case errors.Is(err, optimistic_sync.ErrBlockBeforeNodeHistory) || optimistic_sync.IsCriteriaNotMetError(err):
			return nil, err

		default:
			return nil, fmt.Errorf("unexpected error: %w", err)
		}
	}

	executionResultID := execResultInfo.ExecutionResultID
	snapshot, err := e.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		return nil, fmt.Errorf("failed to find snapshot by execution result ID %s: %w", executionResultID.String(), err)
	}

	executionData, err := snapshot.BlockExecutionData().ByBlockID(ctx, blockID)
	if err != nil {
		return nil, fmt.Errorf("could not find execution data for block %s: %w", blockID, err)
	}

	response := &ExecutionDataResponse{
		Height:        e.height,
		ExecutionData: executionData.BlockExecutionData,
		ExecutorMetadata: accessmodel.ExecutorMetadata{
			ExecutionResultID: executionResultID,
			ExecutorIDs:       execResultInfo.ExecutionNodes.NodeIDs(),
		},
	}

	// prepare criteria for the next call
	e.criteria.ParentExecutionResultID = executionResultID
	e.height += 1

	return response, nil
}
