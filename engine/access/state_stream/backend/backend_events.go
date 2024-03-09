package backend

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
)

type EventsBackend struct {
	log            zerolog.Logger
	broadcaster    *engine.Broadcaster
	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int

	getStartHeight  GetStartHeightFunc
	eventsRetriever EventsRetriever
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

// getResponseFactory returns a function that returns the event response for a given height.
func (b *EventsBackend) getResponseFactory(filter state_stream.EventFilter) GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (response interface{}, err error) {
		eventsResponse, err := b.eventsRetriever.GetAllEventsResponse(ctx, height)
		if err != nil {
			return nil, err
		}

		eventsResponse.Events = filter.Filter(eventsResponse.Events)

		return eventsResponse, nil
	}
}
