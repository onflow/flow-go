package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
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
	broadcaster    *engine.Broadcaster
	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int
	events         storage.Events
	headers        storage.Headers

	getExecutionData GetExecutionDataFunc
	getStartHeight   GetStartHeightFunc

	useIndex bool
}

func (b EventsBackend) SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter state_stream.EventFilter) state_stream.Subscription {
	nextHeight, err := b.getStartHeight(startBlockID, startHeight)
	if err != nil {
		return NewFailedSubscription(err, "could not get start height")
	}

	var responseFactory GetDataByHeightFunc
	if b.useIndex {
		responseFactory = b.getStorageResponseFactory(filter)
	} else {
		responseFactory = b.getExecutionDataResponseFactory(filter)
	}

	sub := NewHeightBasedSubscription(b.sendBufferSize, nextHeight, responseFactory)

	go NewStreamer(b.log, b.broadcaster, b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
}

func (b EventsBackend) getExecutionDataResponseFactory(filter state_stream.EventFilter) GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		executionData, err := b.getExecutionData(ctx, height)
		if err != nil {
			return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
		}

		events := []flow.Event{}
		for _, chunkExecutionData := range executionData.ChunkExecutionDatas {
			events = append(events, filter.Filter(chunkExecutionData.Events)...)
		}

		b.log.Trace().
			Hex("block_id", logging.ID(executionData.BlockID)).
			Uint64("height", height).
			Msgf("sending %d events", len(events))

		return &EventsResponse{
			BlockID: executionData.BlockID,
			Height:  height,
			Events:  events,
		}, nil
	}
}

func (b EventsBackend) getStorageResponseFactory(filter state_stream.EventFilter) GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		header, err := b.headers.ByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("could not get header for height %d: %w", height, err)
		}
		blockID := header.ID()

		events, err := b.events.ByBlockID(blockID)
		if err != nil {
			return nil, fmt.Errorf("could not get events for block %d: %w", height, err)
		}

		events = filter.Filter(events)

		b.log.Trace().
			Hex("block_id", logging.ID(blockID)).
			Uint64("height", height).
			Msgf("sending %d events", len(events))

		return &EventsResponse{
			BlockID: blockID,
			Height:  height,
			Events:  events,
		}, nil
	}
}
