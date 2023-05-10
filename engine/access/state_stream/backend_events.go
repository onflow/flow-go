package state_stream

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
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
}

func (b EventsBackend) SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter EventFilter) Subscription {
	nextHeight, err := b.getStartHeight(startBlockID, startHeight)
	if err != nil {
		sub := NewSubscription(b.sendBufferSize)
		if st, ok := status.FromError(err); ok {
			sub.Fail(status.Errorf(st.Code(), "could not get start height: %s", st.Message()))
			return sub
		}

		sub.Fail(fmt.Errorf("could not get start height: %w", err))
		return sub
	}

	sub := NewHeightBasedSubscription(b.sendBufferSize, nextHeight, b.getResponseFactory(filter))

	go NewStreamer(b.log, b.broadcaster, b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
}

func (b EventsBackend) getResponseFactory(filter EventFilter) GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		header, err := b.headers.ByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("could not get block header for height %d: %w", height, err)
		}

		executionData, err := b.getExecutionData(ctx, header.ID())
		if err != nil {
			return nil, fmt.Errorf("could not get execution data for block %s: %w", header.ID(), err)
		}

		events := []flow.Event{}
		for _, chunkExecutionData := range executionData.ChunkExecutionDatas {
			events = append(events, filter.Filter(chunkExecutionData.Events)...)
		}

		b.log.Trace().
			Hex("block_id", logging.ID(header.ID())).
			Uint64("height", header.Height).
			Msgf("sending %d events", len(events))

		return &EventsResponse{
			BlockID: header.ID(),
			Height:  header.Height,
			Events:  events,
		}, nil
	}
}
