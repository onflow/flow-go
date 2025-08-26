package backend

import (
	"context"
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type EventsBackend struct {
	log zerolog.Logger

	subscriptionHandler  *subscription.SubscriptionHandler
	executionDataTracker tracker.ExecutionDataTracker
	eventsProvider       EventsProvider
}

var _ state_stream.EventsAPI = (*EventsBackend)(nil)

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
func (b *EventsBackend) SubscribeEvents(
	ctx context.Context,
	startBlockID flow.Identifier,
	startHeight uint64,
	filter state_stream.EventFilter,
	execStateQuery entities.ExecutionStateQuery,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeight(ctx, startBlockID, startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getResponseFactory(filter, execStateQuery))
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
func (b *EventsBackend) SubscribeEventsFromStartBlockID(
	ctx context.Context,
	startBlockID flow.Identifier,
	filter state_stream.EventFilter,
	execStateQuery entities.ExecutionStateQuery,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block id")
	}

	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getResponseFactory(filter, execStateQuery))
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
func (b *EventsBackend) SubscribeEventsFromStartHeight(
	ctx context.Context,
	startHeight uint64,
	filter state_stream.EventFilter,
	execStateQuery entities.ExecutionStateQuery,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromHeight(startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block height")
	}

	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getResponseFactory(filter, execStateQuery))
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
func (b *EventsBackend) SubscribeEventsFromLatest(
	ctx context.Context,
	filter state_stream.EventFilter,
	execStateQuery entities.ExecutionStateQuery,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block height")
	}

	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getResponseFactory(filter, execStateQuery))
}

// getResponseFactory returns a function that retrieves the event response for a given height.
//
// Parameters:
// - filter: The event filter used to filter events.
//
// Expected errors during normal operation:
// - subscription.ErrBlockNotReady: execution data for the given block height is not available.
func (b *EventsBackend) getResponseFactory(
	filter state_stream.EventFilter,
	execStateQuery entities.ExecutionStateQuery,
) subscription.GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (response interface{}, err error) {
		// TODO: what should I do with metadata?
		eventsResponse, _, err :=
			b.eventsProvider.GetAllEventsResponse(ctx, height, execStateQuery)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) ||
				errors.Is(err, storage.ErrHeightNotIndexed) {
				return nil, subscription.ErrBlockNotReady
			}
			return nil, fmt.Errorf("block %d is not available yet: %w", height, subscription.ErrBlockNotReady)
		}

		eventsResponse.Events = filter.Filter(eventsResponse.Events)

		return eventsResponse, nil
	}
}
