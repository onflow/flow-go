package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// EventsProvider retrieves events by block height. It can be configured to retrieve events from
// the events indexer(if available) or using a dedicated callback to query it from other sources.
type EventsProvider struct {
	log              zerolog.Logger
	headers          storage.Headers
	getExecutionData GetExecutionDataFunc
	eventsIndex      *index.EventsIndex
	useEventsIndex   bool
}

// GetAllEventsResponse returns a function that retrieves the event response for a given block height.
// Expected errors:
// - codes.NotFound: If block header for the specified block height is not found.
// - error: An error, if any, encountered during getting events from storage or execution data.
func (b *EventsProvider) GetAllEventsResponse(ctx context.Context, height uint64) (*state_stream.EventsResponse, error) {
	var response *state_stream.EventsResponse
	var err error
	if b.useEventsIndex {
		response, err = b.getEventsFromStorage(height)
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
func (b *EventsProvider) getEventsFromExecutionData(ctx context.Context, height uint64) (*state_stream.EventsResponse, error) {
	executionData, err := b.getExecutionData(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
	}

	var events flow.EventsList
	for _, chunkExecutionData := range executionData.ChunkExecutionDatas {
		events = append(events, chunkExecutionData.Events...)
	}

	return &state_stream.EventsResponse{
		BlockID: executionData.BlockID,
		Height:  height,
		Events:  events,
	}, nil
}

// getEventsFromStorage returns the events for a given height from the index storage.
// Expected errors:
// - error: An error indicating any issues with the provided block height or
// an error indicating issue with getting events for a block.
func (b *EventsProvider) getEventsFromStorage(height uint64) (*state_stream.EventsResponse, error) {
	blockID, err := b.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get header for height %d: %w", height, err)
	}

	events, err := b.eventsIndex.ByBlockID(blockID, height)
	if err != nil {
		return nil, fmt.Errorf("could not get events for block %d: %w", height, err)
	}

	return &state_stream.EventsResponse{
		BlockID: blockID,
		Height:  height,
		Events:  events,
	}, nil
}
