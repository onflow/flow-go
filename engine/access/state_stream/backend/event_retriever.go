package backend

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// EventsResponse represents the response containing events for a specific block.
type EventsResponse struct {
	BlockID          flow.Identifier
	Height           uint64
	Events           flow.EventsList
	BlockTimestamp   time.Time
	ExecutorMetadata access.ExecutorMetadata
}

// EventsProvider retrieves events by block height. It can be configured to retrieve events from
// the events indexer(if available) or using a dedicated callback to query it from other sources.
type EventsProvider struct {
	log              zerolog.Logger
	headers          storage.Headers
	execDataProvider ExecutionDataProvider

	fetchFromLocalCache bool
	execResultProvider  optimistic_sync.ExecutionResultInfoProvider
	execStateCache      optimistic_sync.ExecutionStateCache
}

func NewEventsProvider(
	log zerolog.Logger,
	headers storage.Headers,
	execDataProvider ExecutionDataProvider,
	fetchFromLocalCache bool,
	execResultProvider optimistic_sync.ExecutionResultInfoProvider,
	execStateCache optimistic_sync.ExecutionStateCache,
) *EventsProvider {
	return &EventsProvider{
		log:                 log,
		headers:             headers,
		execDataProvider:    execDataProvider,
		fetchFromLocalCache: fetchFromLocalCache,
		execResultProvider:  execResultProvider,
		execStateCache:      execStateCache,
	}
}

// GetAllEventsResponse returns a function that retrieves the event response for a given block height.
// Expected errors:
// - codes.NotFound: If block header for the specified block height is not found.
// - error: An error, if any, encountered during getting events from storage or execution data.
func (b *EventsProvider) GetAllEventsResponse(
	ctx context.Context,
	height uint64,
	criteria optimistic_sync.Criteria,
) (*EventsResponse, error) {
	var response *EventsResponse
	var err error

	if b.fetchFromLocalCache {
		response, err = b.getEventsFromStorage(height, criteria)
	} else {
		response, err = b.getEventsFromExecutionData(ctx, height)
	}

	if err == nil {
		header, err := b.headers.ByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("could not get header for height %d: %w", height, err)
		}
		response.BlockTimestamp = time.UnixMilli(int64(header.Timestamp)).UTC()

		if b.log.GetLevel() == zerolog.TraceLevel {
			b.log.Trace().
				Hex("block_id", logging.ID(response.BlockID)).
				Uint64("height", height).
				Int("events", len(response.Events)).
				Msg("sending events")
		}
	}

	return response, err
}

// getEventsFromExecutionData returns the events for a given height extract from the execution data.
// Expected errors:
// - error: An error indicating issues with getting execution data for block
func (b *EventsProvider) getEventsFromExecutionData(
	ctx context.Context,
	height uint64,
) (*EventsResponse, error) {
	executionData, err := b.execDataProvider.ExecutionData(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
	}

	var events flow.EventsList
	for _, chunkExecutionData := range executionData.ChunkExecutionDatas {
		events = append(events, chunkExecutionData.Events...)
	}

	return &EventsResponse{
		BlockID: executionData.BlockID,
		Height:  height,
		Events:  events,
	}, nil
}

// getEventsFromStorage returns the events for a given height from the index storage.
// Expected errors:
// - error: An error indicating any issues with the provided block height or
// an error indicating issue with getting events for a block.
func (b *EventsProvider) getEventsFromStorage(
	height uint64,
	criteria optimistic_sync.Criteria,
) (*EventsResponse, error) {
	blockID, err := b.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get header for height %d: %w", height, err)
	}

	result, err := b.execResultProvider.ExecutionResultInfo(
		blockID,
		criteria,
	)
	if err != nil {
		return &EventsResponse{}, fmt.Errorf("error fetching execution result: %w", err)
	}

	snapshot, err := b.execStateCache.Snapshot(result.ExecutionResultID)
	if err != nil {
		return &EventsResponse{},
			fmt.Errorf(
				"failed to get snapshot for execution result %s: %w",
				result.ExecutionResultID,
				err,
			)
	}

	events, err := snapshot.Events().ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get events for block %d: %w", blockID, err)
	}

	// Normalize ordering for events coming from storage.
	// The index may return events ordered by TransactionID (implementation detail),
	// but consumers and tests expect events ordered by transaction index and event index.
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].TransactionIndex == events[j].TransactionIndex {
			return events[i].EventIndex < events[j].EventIndex
		}
		return events[i].TransactionIndex < events[j].TransactionIndex
	})

	metadata := access.ExecutorMetadata{
		ExecutionResultID: result.ExecutionResultID,
		ExecutorIDs:       result.ExecutionNodes.NodeIDs(),
	}

	return &EventsResponse{
		BlockID:          blockID,
		Height:           height,
		Events:           events,
		ExecutorMetadata: metadata,
	}, nil
}
