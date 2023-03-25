package state_stream

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type EventsResponse struct {
	BlockID flow.Identifier
	Height  uint64
	Events  flow.EventsList
}

type EventsBackend struct {
	log         zerolog.Logger
	headers     storage.Headers
	broadcaster *engine.Broadcaster
	sendTimeout time.Duration

	getExecutionData GetExecutionDataFunc
	getStartHeight   GetStartHeightFunc
}

func (b EventsBackend) SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter EventFilter) Subscription {
	sub := NewHeightBasedSubscription(b.getResponseFactory(filter))

	nextHeight, err := b.getStartHeight(startBlockID, startHeight)
	if err != nil {
		sub.Fail(fmt.Errorf("could not get start height: %w", err))
		return sub
	}

	sub.nextHeight = nextHeight

	go NewStreamer(b.log, b.broadcaster, b.sendTimeout, sub).Stream(ctx)

	return sub
}

func (b EventsBackend) getResponseFactory(filter EventFilter) GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		header, err := b.headers.ByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("could not get block header: %w", err)
		}

		executionData, err := b.getExecutionData(ctx, header.ID())
		if err != nil {
			return nil, err
		}

		events := []flow.Event{}
		for _, chunkExecutionData := range executionData.ChunkExecutionDatas {
			events = append(events, filter.Filter(chunkExecutionData.Events)...)
		}

		b.log.Debug().Msgf("sending %d events", len(events))

		return &EventsResponse{
			BlockID: header.ID(),
			Height:  header.Height,
			Events:  events,
		}, nil
	}
}
