package backend

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

type EventsResponse struct {
	BlockID flow.Identifier
	Height  uint64
	Events  flow.EventsList
}

type EventsRetriever struct {
	log              zerolog.Logger
	headers          storage.Headers
	getExecutionData GetExecutionDataFunc
	eventsIndex      *index.EventsIndex
	useEventsIndex   bool
}

// GetAllEventsResponse returns a function that returns the event response for a given height.
func (b *EventsRetriever) GetAllEventsResponse(ctx context.Context, height uint64) (*EventsResponse, error) {
	var response *EventsResponse
	var err error
	if b.useEventsIndex {
		response, err = b.getEventsFromStorage(height)
	} else {
		response, err = b.getEventsFromExecutionData(ctx, height)
	}

	if err == nil && b.log.GetLevel() == zerolog.TraceLevel {
		b.log.Trace().
			Hex("block_id", logging.ID(response.BlockID)).
			Uint64("height", height).
			Int("events", len(response.Events)).
			Msg("sending events")
	}

	return response, err
}

// getEventsFromExecutionData returns the events for a given height extract from the execution data.
func (b *EventsRetriever) getEventsFromExecutionData(ctx context.Context, height uint64) (*EventsResponse, error) {
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
func (b *EventsRetriever) getEventsFromStorage(height uint64) (*EventsResponse, error) {
	blockID, err := b.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get header for height %d: %w", height, err)
	}

	events, err := b.eventsIndex.ByBlockID(blockID, height)
	if err != nil {
		return nil, fmt.Errorf("could not get events for block %d: %w", height, err)
	}

	return &EventsResponse{
		BlockID: blockID,
		Height:  height,
		Events:  events,
	}, nil
}
