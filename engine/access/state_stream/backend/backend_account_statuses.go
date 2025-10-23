package backend

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/storage"
)

type AccountStatusesResponse struct {
	BlockID          flow.Identifier
	Height           uint64
	AccountEvents    map[string]flow.EventsList
	ExecutorMetadata access.ExecutorMetadata
}

// AccountStatusesBackend is a struct representing a backend implementation for subscribing to account statuses changes.
type AccountStatusesBackend struct {
	log                 zerolog.Logger
	subscriptionFactory *subscription.Factory

	executionDataTracker tracker.ExecutionDataTracker
	eventsProvider       *EventsProvider
}

var _ state_stream.AccountsAPI = (*AccountStatusesBackend)(nil)

// subscribe creates and returns a subscription to receive account status updates starting from the specified height.
func (b *AccountStatusesBackend) subscribe(
	ctx context.Context,
	nextHeight uint64,
	filter state_stream.AccountStatusFilter,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	return b.subscriptionFactory.CreateHeightBasedSubscription(ctx, nextHeight, b.getAccountStatusResponseFactory(filter, criteria))
}

// SubscribeAccountStatusesFromStartBlockID subscribes to the streaming of account status changes starting from
// a specific block ID with an optional status filter.
// Errors:
// - codes.ErrNotFound if could not get block by start blockID.
// - codes.Internal if there is an internal error.
func (b *AccountStatusesBackend) SubscribeAccountStatusesFromStartBlockID(
	ctx context.Context,
	startBlockID flow.Identifier,
	filter state_stream.AccountStatusFilter,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block id")
	}
	return b.subscribe(ctx, nextHeight, filter, criteria)
}

// SubscribeAccountStatusesFromStartHeight subscribes to the streaming of account status changes starting from
// a specific block height, with an optional status filter.
// Errors:
// - codes.ErrNotFound if could not get block by start height.
// - codes.Internal if there is an internal error.
func (b *AccountStatusesBackend) SubscribeAccountStatusesFromStartHeight(
	ctx context.Context,
	startHeight uint64,
	filter state_stream.AccountStatusFilter,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromHeight(startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from block height")
	}
	return b.subscribe(ctx, nextHeight, filter, criteria)
}

// SubscribeAccountStatusesFromLatestBlock subscribes to the streaming of account status changes starting from a
// latest sealed block, with an optional status filter.
//
// No errors are expected during normal operation.
func (b *AccountStatusesBackend) SubscribeAccountStatusesFromLatestBlock(
	ctx context.Context,
	filter state_stream.AccountStatusFilter,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height from latest")
	}
	return b.subscribe(ctx, nextHeight, filter, criteria)
}

// getAccountStatusResponseFactory returns a function that returns the account statuses response for a given height.
//
// Errors:
// - subscription.ErrBlockNotReady: If block header for the specified block height is not found.
// - error: An error, if any, encountered during getting events from storage or execution data.
func (b *AccountStatusesBackend) getAccountStatusResponseFactory(
	filter state_stream.AccountStatusFilter,
	criteria optimistic_sync.Criteria,
) subscription.GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		eventsResponse, err := b.eventsProvider.Events(ctx, height, criteria)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) ||
				errors.Is(err, storage.ErrHeightNotIndexed) {
				return nil, fmt.Errorf("block %d is not available yet: %w", height, subscription.ErrBlockNotReady)
			}
			return nil, err
		}
		filteredProtocolEvents := filter.Filter(eventsResponse.Events)
		allAccountProtocolEvents := filter.GroupCoreEventsByAccountAddress(filteredProtocolEvents, b.log)

		return &AccountStatusesResponse{
			BlockID:          eventsResponse.BlockID,
			Height:           eventsResponse.Height,
			AccountEvents:    allAccountProtocolEvents,
			ExecutorMetadata: eventsResponse.ExecutorMetadata,
		}, nil
	}
}
