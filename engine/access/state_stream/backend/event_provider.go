package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// eventProvider is responsible for managing event-related data and interactions within the protocol state.
// It interacts with multiple components, such as protocol state, execution results, and event filters.
//
// NOT CONCURRENCY SAFE! eventProvider is designed to be used by a single streamer goroutine.
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
	blockID, err := e.headers.BlockIDByHeight(e.height)
	if err != nil {
		return nil, errors.Join(subscription.ErrBlockNotReady, err)
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

	// the spork root block will never have execution data available. If requested, return an empty result.
	sporkRootBlock := e.state.Params().SporkRootBlock()
	if e.height == sporkRootBlock.Height {
		response := &EventsResponse{
			BlockID: sporkRootBlock.ID(),
			Height:  e.height,
		}

		// prepare criteria for the next call
		e.criteria.ParentExecutionResultID = execResultInfo.ExecutionResultID
		e.height += 1

		return response, nil
	}

	executionStateSnapshot, err := e.executionStateCache.Snapshot(execResultInfo.ExecutionResultID)
	if err != nil {
		return nil, fmt.Errorf("failed to find execution state snapshot by execution result ID %s: %w",
			execResultInfo.ExecutionResultID.String(), err)
	}

	events, err := executionStateSnapshot.Events().ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not find events for block %s: %w", blockID, err)
	}

	response := &EventsResponse{
		BlockID: blockID,
		Height:  e.height,
		Events:  e.eventFilter.Filter(events),
	}

	// prepare criteria for the next call
	e.criteria.ParentExecutionResultID = execResultInfo.ExecutionResultID
	e.height += 1

	return response, nil
}
