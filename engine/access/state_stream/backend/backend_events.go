package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
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

	getExecutionData GetExecutionDataFunc
	getStartHeight   GetStartHeightFunc
}

func (b EventsBackend) SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter state_stream.EventFilter) state_stream.Subscription {
	nextHeight, err := b.getStartHeight(startBlockID, startHeight)
	if err != nil {
		return NewFailedSubscription(err, "could not get start height")
	}

	sub := NewHeightBasedSubscription(b.sendBufferSize, nextHeight, b.getResponseFactory(filter))

	go NewStreamer(b.log, b.broadcaster, b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
}

func (b EventsBackend) getResponseFactory(filter state_stream.EventFilter) GetDataByHeightFunc {
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
