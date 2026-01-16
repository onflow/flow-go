package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type eventProvider struct {
	state                   protocol.State
	headers                 storage.Headers
	executionDataTracker    tracker.ExecutionDataTracker
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider
	executionStateCache     optimistic_sync.ExecutionStateCache
	criteria                optimistic_sync.Criteria
	height                  uint64
	eventFilter             state_stream.EventFilter
}

func newEventProvider(
	state protocol.State,
	headers storage.Headers,
	executionDataTracker tracker.ExecutionDataTracker,
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider,
	executionStateCache optimistic_sync.ExecutionStateCache,
	nextCriteria optimistic_sync.Criteria,
	startHeight uint64,
	eventFilter state_stream.EventFilter,
) *eventProvider {
	return &eventProvider{
		state:                   state,
		headers:                 headers,
		executionDataTracker:    executionDataTracker,
		executionResultProvider: executionResultProvider,
		executionStateCache:     executionStateCache,
		criteria:                nextCriteria,
		height:                  startHeight,
		eventFilter:             eventFilter,
	}
}

var _ subscription.DataProvider = (*eventProvider)(nil)

func (e *eventProvider) NextData(_ context.Context) (any, error) {
	availableFinalizedHeight := e.executionDataTracker.GetHighestAvailableFinalizedHeight()
	if e.height > availableFinalizedHeight {
		// fail early if no notification has been received for the given block height.
		// note: it's possible for the data to exist in the data store before the notification is
		// received. this ensures a consistent view is available to all streams.
		return nil, subscription.ErrBlockNotReady
	}

	// the spork root block will never have execution data available. If requested, return an empty result.
	if e.height == e.state.Params().SporkRootBlockHeight() {
		response := &EventsResponse{
			BlockID: e.state.Params().SporkRootBlock().ID(),
			Height:  e.height,
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

	events, err := snapshot.Events().ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not find events for block %s: %w", blockID, err)
	}

	response := &EventsResponse{
		BlockID:        blockID,
		Height:         e.height,
		Events:         e.eventFilter.Filter(events),
		BlockTimestamp: time.Time{}, // TODO: do we even set this?
	}

	// prepare criteria for the next call
	e.criteria.ParentExecutionResultID = executionResultID
	e.height += 1

	return response, nil
}
