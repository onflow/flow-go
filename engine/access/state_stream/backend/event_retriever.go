package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// EventsResponse represents the response containing events for a specific block.
type EventsResponse struct {
	BlockID        flow.Identifier
	Height         uint64
	Events         flow.EventsList
	BlockTimestamp time.Time
}

// EventsProvider retrieves events by block height. It can be configured to retrieve events from
// the events indexer(if available) or using a dedicated callback to query it from other sources.
type EventsProvider struct {
	log              zerolog.Logger
	headers          storage.Headers
	getExecutionData GetExecutionDataFunc

	fetchFromLocalCache bool
	execResultProvider  optimistic_sync.ExecutionResultProvider
	execStateCache      optimistic_sync.ExecutionStateCache
	operatorCriteria    optimistic_sync.Criteria
}

// GetAllEventsResponse returns a function that retrieves the event response for a given block height.
// Expected errors:
// - codes.NotFound: If block header for the specified block height is not found.
// - error: An error, if any, encountered during getting events from storage or execution data.
func (b *EventsProvider) GetAllEventsResponse(
	ctx context.Context,
	height uint64,
	execStateQuery entities.ExecutionStateQuery,
) (*EventsResponse, entities.ExecutorMetadata, error) {
	var response *EventsResponse
	var err error
	var metadata entities.ExecutorMetadata

	if b.fetchFromLocalCache {
		response, metadata, err = b.getEventsFromStorage(height, execStateQuery)
	} else {
		response, err = b.getEventsFromExecutionData(ctx, height)
	}

	if err == nil {
		header, err := b.headers.ByHeight(height)
		if err != nil {
			return nil, metadata, fmt.Errorf("could not get header for height %d: %w", height, err)
		}
		response.BlockTimestamp = header.Timestamp

		if b.log.GetLevel() == zerolog.TraceLevel {
			b.log.Trace().
				Hex("block_id", logging.ID(response.BlockID)).
				Uint64("height", height).
				Int("events", len(response.Events)).
				Msg("sending events")
		}
	}

	return response, metadata, err
}

// getEventsFromExecutionData returns the events for a given height extract from the execution data.
// Expected errors:
// - error: An error indicating issues with getting execution data for block
func (b *EventsProvider) getEventsFromExecutionData(ctx context.Context, height uint64) (*EventsResponse, error) {
	executionData, err := b.getExecutionData(ctx, height)
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
	executionState entities.ExecutionStateQuery,
) (*EventsResponse, entities.ExecutorMetadata, error) {
	metadata := entities.ExecutorMetadata{}
	blockID, err := b.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, metadata, fmt.Errorf("could not get header for height %d: %w", height, err)
	}

	clientCriteria := optimistic_sync.Criteria{
		AgreeingExecutorsCount: uint(executionState.AgreeingExecutorsCount),
		RequiredExecutors:      convert.MessagesToIdentifiers(executionState.RequiredExecutorId),
	}

	result, err := b.execResultProvider.ExecutionResult(
		blockID,
		b.operatorCriteria.OverrideWith(clientCriteria),
	)
	if err != nil {
		return &EventsResponse{}, metadata, fmt.Errorf("error fetching execution result: %w", err)
	}

	metadata = entities.ExecutorMetadata{
		ExecutionResultId: convert.IdentifierToMessage(result.ExecutionResult.ID()),
		ExecutorId:        convert.IdentifiersToMessages(result.ExecutionNodes.NodeIDs()),
	}

	snapshot, err := b.execStateCache.Snapshot(result.ExecutionResult.ID())
	if err != nil {
		return &EventsResponse{}, metadata,
			fmt.Errorf("failed to get snapshot for execution result %s: %w", result.ExecutionResult.ID(), err)
	}

	events, err := snapshot.Events().ByBlockID(blockID)
	if err != nil {
		return nil, metadata, fmt.Errorf("could not get events for block %d: %w", blockID, err)
	}

	return &EventsResponse{
		BlockID: blockID,
		Height:  height,
		Events:  events,
	}, metadata, nil
}
