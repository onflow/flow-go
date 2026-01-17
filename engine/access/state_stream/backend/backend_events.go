package backend

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type EventsBackend struct {
	log                     zerolog.Logger
	state                   protocol.State
	headers                 storage.Headers
	nodeRootBlock           *flow.Header
	executionDataTracker    tracker.ExecutionDataTracker
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider
	executionStateCache     optimistic_sync.ExecutionStateCache
	subscriptionFactory     *subscription.SubscriptionHandler
}

func NewEventsBackend(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	nodeRootBlock *flow.Header,
	executionDataTracker tracker.ExecutionDataTracker,
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider,
	executionStateCache optimistic_sync.ExecutionStateCache,
	subscriptionFactory *subscription.SubscriptionHandler,
) *EventsBackend {
	return &EventsBackend{
		log:                     log,
		state:                   state,
		headers:                 headers,
		nodeRootBlock:           nodeRootBlock,
		executionDataTracker:    executionDataTracker,
		executionResultProvider: executionResultProvider,
		executionStateCache:     executionStateCache,
		subscriptionFactory:     subscriptionFactory,
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
	startBlockHeight uint64,
	eventFilter state_stream.EventFilter,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	if startBlockID != flow.ZeroID && startBlockHeight == 0 {
		return b.SubscribeEventsFromStartBlockID(ctx, startBlockID, eventFilter, criteria)
	}

	if startBlockHeight > 0 && startBlockID == flow.ZeroID {
		return b.SubscribeEventsFromStartHeight(ctx, startBlockHeight, eventFilter, criteria)
	}

	return subscription.NewFailedSubscription(nil, "one of start block ID and start block height must be provided")
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
	eventFilter state_stream.EventFilter,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	snapshot := b.state.AtBlockID(startBlockID)

	// check if the block header for the given block is available in the storage
	header, err := snapshot.Head()
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get header for start block")
	}

	availableExecutors, err := snapshot.Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not retrieve available executors")
	}

	err = criteria.Validate(availableExecutors)
	if err != nil {
		return subscription.NewFailedSubscription(err, "criteria validation failed")
	}

	snapshotBuilder := newExecutionStateSnapshotBuilder(
		b.headers,
		b.executionResultProvider,
		b.executionStateCache,
	)

	eventProvider := newEventProvider(
		b.state,
		snapshotBuilder,
		criteria,
		header.Height,
		eventFilter,
	)

	return b.subscriptionFactory.Subscribe(ctx, eventProvider)
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
	startBlockHeight uint64,
	eventFilter state_stream.EventFilter,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	if startBlockHeight < b.nodeRootBlock.Height {
		return subscription.NewFailedSubscription(nil, "start height is below the node's range of available blocks.")
	}

	snapshot := b.state.AtHeight(startBlockHeight)

	// check if the block header for the given block is available in the storage
	header, err := snapshot.Head()
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get header for start block")
	}

	availableExecutors, err := snapshot.Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not retrieve available executors")
	}

	err = criteria.Validate(availableExecutors)
	if err != nil {
		return subscription.NewFailedSubscription(err, "criteria validation failed")
	}

	snapshotBuilder := newExecutionStateSnapshotBuilder(
		b.headers,
		b.executionResultProvider,
		b.executionStateCache,
	)

	eventProvider := newEventProvider(
		b.state,
		snapshotBuilder,
		criteria,
		header.Height,
		eventFilter,
	)

	return b.subscriptionFactory.Subscribe(ctx, eventProvider)
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
	eventFilter state_stream.EventFilter,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	header, err := b.state.Sealed().Head()
	if err != nil {
		// In the RPC engine, if we encounter an error from the protocol state indicating state corruption,
		// we should halt processing requests
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return subscription.NewFailedSubscription(err, "failed to lookup sealed header")
	}

	return b.SubscribeEventsFromStartHeight(ctx, header.Height, eventFilter, criteria)
}
