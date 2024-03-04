package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	rpcbackend "github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

type EventsResponse struct {
	BlockID flow.Identifier
	Height  uint64
	Events  flow.EventsList
}

type EventsBackend struct {
	log            zerolog.Logger
	headers        storage.Headers
	broadcaster    *engine.Broadcaster
	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int

	getExecutionData GetExecutionDataFunc
	getStartHeight   GetStartHeightFunc

	useIndex    bool
	eventsIndex *rpcbackend.EventsIndex
}

func (b *EventsBackend) SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter state_stream.EventFilter) state_stream.Subscription {
	nextHeight, err := b.getStartHeight(startBlockID, startHeight)
	if err != nil {
		return NewFailedSubscription(err, "could not get start height")
	}

	sub := NewHeightBasedSubscription(b.sendBufferSize, nextHeight, b.getResponseFactory(filter))

	go NewStreamer(b.log, b.broadcaster, b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
}

// getResponseFactory returns a function function that returns the event response for a given height.
func (b *EventsBackend) getResponseFactory(filter state_stream.EventFilter) GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (response interface{}, err error) {
		if b.useIndex {
			response, err = b.getEventsFromStorage(height, filter)
		} else {
			response, err = b.getEventsFromExecutionData(ctx, height, filter)
		}

		if err == nil && b.log.GetLevel() == zerolog.TraceLevel {
			eventsResponse := response.(*EventsResponse)
			b.log.Trace().
				Hex("block_id", logging.ID(eventsResponse.BlockID)).
				Uint64("height", height).
				Int("events", len(eventsResponse.Events)).
				Msg("sending events")
		}
		return
	}
}

// getEventsFromExecutionData returns the events for a given height extractd from the execution data.
func (b *EventsBackend) getEventsFromExecutionData(ctx context.Context, height uint64, filter state_stream.EventFilter) (*EventsResponse, error) {
	executionData, err := b.getExecutionData(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
	}

	var events flow.EventsList
	for _, chunkExecutionData := range executionData.ChunkExecutionDatas {
		events = append(events, filter.Filter(chunkExecutionData.Events)...)
	}

	return &EventsResponse{
		BlockID: executionData.BlockID,
		Height:  height,
		Events:  events,
	}, nil
}

// getEventsFromStorage returns the events for a given height from the index storage.
func (b *EventsBackend) getEventsFromStorage(height uint64, filter state_stream.EventFilter) (*EventsResponse, error) {
	blockID, err := b.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get header for height %d: %w", height, err)
	}

	events, err := b.eventsIndex.ByBlockID(blockID, height)
	if err != nil {
		return nil, fmt.Errorf("could not get events for block %d: %w", height, err)
	}

	b.log.Trace().
		Uint64("height", height).
		Hex("block_id", logging.ID(blockID)).
		Int("events", len(events)).
		Msg("events from storage")

	return &EventsResponse{
		BlockID: blockID,
		Height:  height,
		Events:  filter.Filter(events),
	}, nil
}
