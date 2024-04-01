package backend

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/storage"
)

type SubscribeEventsResponse struct {
	EventsResponse
	BlockTimestamp time.Time
	MessageIndex   uint64
}

type EventsBackend struct {
	log zerolog.Logger

	headers              storage.Headers
	subscriptionHandler  *subscription.SubscriptionHandler
	executionDataTracker subscription.ExecutionDataTracker
	eventsRetriever      EventsRetriever
}

// SubscribeEvents is deprecated and will be removed in a future version.
// Use SubscribeEventsFromStartBlockID, SubscribeEventsFromStartHeight or SubscribeEventsFromLatest.
//
// SubscribeEvents streams events for all blocks starting at the specified block ID or block height
// up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Only one of startBlockID and startHeight may be set. If neither startBlockID nor startHeight is provided,
// the latest sealed block is used.
//
// Events within each block are filtered by the provided EventFilter, and only
// those events that match the filter are returned. If no filter is provided,
// all events are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block. If provided, startHeight should be 0.
// - startHeight: The height of the starting block. If provided, startBlockID should be flow.ZeroID.
// - filter: The event filter used to filter events.
//
// If invalid parameters will be supplied SubscribeEvents will return a failed subscription.
func (b *EventsBackend) SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter state_stream.EventFilter) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeight(ctx, startBlockID, startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	messageIndex := counters.NewMonotonousCounter(0)
	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getResponseFactory(filter, &messageIndex))
}

// SubscribeEventsFromStartBlockID streams events starting at the specified block ID,
// up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Events within each block are filtered by the provided EventFilter, and only
// those events that match the filter are returned. If no filter is provided,
// all events are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block.
// - filter: The event filter used to filter events.
//
// If invalid parameters will be supplied SubscribeEventsFromStartBlockID will return a failed subscription.
func (b *EventsBackend) SubscribeEventsFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, filter state_stream.EventFilter) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block id")
	}

	return b.subscribeEvents(ctx, nextHeight, filter)
}

// SubscribeEventsFromStartHeight streams events starting at the specified block height,
// up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Events within each block are filtered by the provided EventFilter, and only
// those events that match the filter are returned. If no filter is provided,
// all events are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - startHeight: The height of the starting block.
// - filter: The event filter used to filter events.
//
// If invalid parameters will be supplied SubscribeEventsFromStartHeight will return a failed subscription.
func (b *EventsBackend) SubscribeEventsFromStartHeight(ctx context.Context, startHeight uint64, filter state_stream.EventFilter) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromHeight(startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block height")
	}

	return b.subscribeEvents(ctx, nextHeight, filter)
}

// SubscribeEventsFromLatest subscribes to events starting at the latest sealed block,
// up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Events within each block are filtered by the provided EventFilter, and only
// those events that match the filter are returned. If no filter is provided,
// all events are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - filter: The event filter used to filter events.
//
// If invalid parameters will be supplied SubscribeEventsFromLatest will return a failed subscription.
func (b *EventsBackend) SubscribeEventsFromLatest(ctx context.Context, filter state_stream.EventFilter) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block height")
	}

	return b.subscribeEvents(ctx, nextHeight, filter)
}

// subscribeEvents is a helper function that subscribes to events starting at the specified height,
// filtered by the provided event filter.
//
// Parameters:
// - ctx: Context for the operation.
// - nextHeight: The height of the starting block.
// - filter: The event filter used to filter events.
//
// No errors are expected during normal operation.
func (b *EventsBackend) subscribeEvents(ctx context.Context, nextHeight uint64, filter state_stream.EventFilter) subscription.Subscription {
	messageIndex := counters.NewMonotonousCounter(0)
	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getResponseFactory(filter, &messageIndex))
}

// getResponseFactory returns a function that retrieves the event response for a given height.
//
// Parameters:
// - filter: The event filter used to filter events.
// - index: A strict monotonous counter used to track the message index.
//
// Expected errors during normal operation:
// - codes.NotFound: If block header for the specified block height is not found, if events for the specified block height are not found.
// - codes.Internal: If the message index has already been incremented.
func (b *EventsBackend) getResponseFactory(filter state_stream.EventFilter, index *counters.StrictMonotonousCounter) subscription.GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (response interface{}, err error) {
		eventsResponse, err := b.eventsRetriever.GetAllEventsResponse(ctx, height)
		if err != nil {
			return nil, err
		}

		header, err := b.headers.ByHeight(height)
		if err != nil {
			return nil, rpc.ConvertStorageError(err)
		}

		messageIndex := index.Value()
		if ok := index.Set(messageIndex + 1); !ok {
			return nil, status.Errorf(codes.Internal, "the message index has already been incremented to %d", index.Value())
		}

		subscribeEventsResponse := &SubscribeEventsResponse{
			EventsResponse: EventsResponse{
				BlockID: eventsResponse.BlockID,
				Height:  eventsResponse.Height,
				Events:  filter.Filter(eventsResponse.Events),
			},
			BlockTimestamp: header.Timestamp,
			MessageIndex:   messageIndex,
		}

		return subscribeEventsResponse, nil
	}
}
