package backend

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/storage"
)

type EventsBackend struct {
	log zerolog.Logger

	subscriptionFactory  *subscription.Factory
	executionDataTracker tracker.ExecutionDataTracker
	eventsProvider       *EventsProvider
}

var _ state_stream.EventsAPI = (*EventsBackend)(nil)

func NewEventsBackend(
	log zerolog.Logger,
	subscriptionFactory *subscription.Factory,
	executionDataTracker tracker.ExecutionDataTracker,
	eventsProvider *EventsProvider,
) EventsBackend {
	return EventsBackend{
		log:                  log,
		subscriptionFactory:  subscriptionFactory,
		executionDataTracker: executionDataTracker,
		eventsProvider:       eventsProvider,
	}
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
func (b *EventsBackend) SubscribeEvents(
	ctx context.Context,
	startBlockID flow.Identifier,
	startHeight uint64,
	filter state_stream.EventFilter,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeight(ctx, startBlockID, startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	return b.subscriptionFactory.CreateHeightBasedSubscription(
		ctx,
		nextHeight,
		b.createResponseHandler(filter, criteria),
	)
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
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block id")
	}

	return b.subscriptionFactory.CreateHeightBasedSubscription(
		ctx,
		nextHeight,
		b.createResponseHandler(filter, criteria),
	)
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
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromHeight(startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block height")
	}
	return b.subscriptionFactory.CreateHeightBasedSubscription(
		ctx,
		nextHeight,
		b.createResponseHandler(filter, criteria),
	)
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
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block height")
	}

	return b.subscriptionFactory.CreateHeightBasedSubscription(ctx, nextHeight, b.createResponseHandler(filter, criteria))
}

// createResponseHandler constructs a height-indexed response producer for event streaming.
//
// It returns a closure compatible with height-based streaming that, for each requested
// block height, fetches the block’s events, applies the provided filter and returns a response
// The closure preserves fork affinity across sequential calls by capturing and propagating
// the previous call’s ExecutionResultID into the optimistic_sync.Criteria as
// ParentExecutionResultID for the next call. This scoping ensures that lineage is maintained
// per stream without relying on backend-global mutable state.
//
// Concurrency and lifecycle:
//   - The returned closure maintains an internal state (nextCriteria) and is intended for sequential
//     invocation within a single subscription/stream. It should not be used concurrently from
//     multiple goroutines.
//
// Errors:
// - Returns subscription.ErrBlockNotReady when the requested block’s data is not yet available.
func (b *EventsBackend) createResponseHandler(
	filter state_stream.EventFilter,
	criteria optimistic_sync.Criteria,
) subscription.GetDataByHeightFunc {
	// Make a private copy of the initial criteria and thread it across invocations of the
	// returned closure. On each successful fetch we advance lineage by setting
	// ParentExecutionResultID for the next call to preserve fork affinity within this stream.
	nextCriteria := criteria

	return func(ctx context.Context, height uint64) (response interface{}, err error) {
		// Fetch events for the requested height using the current criteria. If a
		// ParentExecutionResultID was captured previously, this biases the lookup toward the
		// same execution fork.
		eventsResponse, err := b.eventsProvider.Events(ctx, height, nextCriteria)
		if err != nil {
			// Normalize "not yet available" conditions so the streaming layer can retry/back off.
			if errors.Is(err, storage.ErrNotFound) ||
				errors.Is(err, storage.ErrHeightNotIndexed) {
				return nil, subscription.ErrBlockNotReady
			}

			// For any other error, return a consistent NotReady wrapper to callers.
			return nil, fmt.Errorf("block %d is not available yet: %w", height, subscription.ErrBlockNotReady)
		}

		// Advance lineage only after a successful fetch: carry forward the current block’s
		// ExecutionResultID so the next call remains on the same execution fork.
		nextCriteria.ParentExecutionResultID = eventsResponse.ExecutorMetadata.ExecutionResultID

		eventsResponse.Events = filter.Filter(eventsResponse.Events)
		return eventsResponse, nil
	}
}
