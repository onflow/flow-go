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

type AccountStatusesResponse struct {
	BlockID       flow.Identifier
	Height        uint64
	AccountEvents map[string]flow.EventsList
}

// AccountStatusesBackend is a struct representing a backend implementation for subscribing to account statuses changes.
type AccountStatusesBackend struct {
	log                     zerolog.Logger
	state                   protocol.State
	headers                 storage.Headers
	sporkRootBlock          *flow.Block
	executionDataTracker    tracker.ExecutionDataTracker
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider
	executionStateCache     optimistic_sync.ExecutionStateCache
	subscriptionFactory     *subscription.SubscriptionHandler
}

func NewAccountStatusesBackend(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	sporkRootBlock *flow.Block,
	executionDataTracker tracker.ExecutionDataTracker,
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider,
	executionStateCache optimistic_sync.ExecutionStateCache,
	subscriptionFactory *subscription.SubscriptionHandler,
) *AccountStatusesBackend {
	return &AccountStatusesBackend{
		log:                     log,
		state:                   state,
		headers:                 headers,
		sporkRootBlock:          sporkRootBlock,
		executionDataTracker:    executionDataTracker,
		executionResultProvider: executionResultProvider,
		executionStateCache:     executionStateCache,
		subscriptionFactory:     subscriptionFactory,
	}
}

// SubscribeAccountStatuses is deprecated and will be removed in a future version.
// Use SubscribeAccountStatusesFromStartBlockID, SubscribeAccountStatusesFromStartHeight or SubscribeAccountStatusesFromLatestBlock.
//
// SubscribeAccountStatuses streams account status changes for all blocks starting at the specified block ID or block height
// up until the latest available block. Once the latest is
// reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Only one of startBlockID and startHeight may be set. If neither startBlockID nor startHeight is provided,
// the latest sealed block is used.
//
// Account statuses within each block are filtered by the provided AccountStatusFilter, and only
// those that match the filter are returned. If no filter is provided,
// all account statuses are returned.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block. If provided, startHeight should be 0.
// - startHeight: The height of the starting block. If provided, startBlockID should be flow.ZeroID.
// - filter: The account status filter used to filter account statuses.
//
// If invalid parameters will be supplied SubscribeAccountStatuses will return a failed subscription.
func (b *AccountStatusesBackend) SubscribeAccountStatuses(
	ctx context.Context,
	startBlockID flow.Identifier,
	startBlockHeight uint64,
	statusFilter state_stream.AccountStatusFilter,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	if startBlockID != flow.ZeroID && startBlockHeight == 0 {
		return b.SubscribeAccountStatusesFromStartBlockID(ctx, startBlockID, statusFilter, criteria)
	}

	if startBlockHeight > 0 && startBlockID == flow.ZeroID {
		return b.SubscribeAccountStatusesFromStartHeight(ctx, startBlockHeight, statusFilter, criteria)
	}

	return subscription.NewFailedSubscription(nil, "one of start block ID and start block height must be provided")
}

// SubscribeAccountStatusesFromStartBlockID subscribes to the streaming of account status changes starting from
// a specific block ID with an optional status filter.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block.
// - filter: The account status filter used to filter account statuses.
//
// If invalid parameters will be supplied SubscribeAccountStatusesFromStartBlockID will return a failed subscription.
func (b *AccountStatusesBackend) SubscribeAccountStatusesFromStartBlockID(
	ctx context.Context,
	startBlockID flow.Identifier,
	statusFilter state_stream.AccountStatusFilter,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	// check if the block header for the given block ID is available in the storage
	header, err := b.headers.ByBlockID(startBlockID)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get header for block height")
	}

	if header.Height < b.sporkRootBlock.Height {
		return subscription.NewFailedSubscription(err, "block height is less than the spork root block")
	}

	availableExecutors, err :=
		b.state.AtHeight(header.Height).Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not retrieve available executors")
	}

	err = criteria.Validate(availableExecutors)
	if err != nil {
		return subscription.NewFailedSubscription(err, "criteria validation failed")
	}

	executionDataProvider := newExecutionDataProvider(
		b.state,
		b.headers,
		b.executionDataTracker,
		b.executionResultProvider,
		b.executionStateCache,
		criteria,
		header.Height,
	)
	accountStatusProvider := newAccountStatusProvider(b.log, executionDataProvider, statusFilter)

	return b.subscriptionFactory.Subscribe(ctx, accountStatusProvider)
}

// SubscribeAccountStatusesFromStartHeight subscribes to the streaming of account status changes starting from
// a specific block height, with an optional status filter.
//
// Parameters:
// - ctx: Context for the operation.
// - startHeight: The height of the starting block.
// - filter: The account status filter used to filter account statuses.
//
// If invalid parameters will be supplied SubscribeAccountStatusesFromStartHeight will return a failed subscription.
func (b *AccountStatusesBackend) SubscribeAccountStatusesFromStartHeight(
	ctx context.Context,
	startBlockHeight uint64,
	statusFilter state_stream.AccountStatusFilter,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	if startBlockHeight < b.sporkRootBlock.Height {
		return subscription.NewFailedSubscription(nil,
			"start height must be greater than or equal to the spork root height")
	}

	// check if the block header for the given height is available in the storage
	header, err := b.headers.ByHeight(startBlockHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "error getting block header by height")
	}

	availableExecutors, err :=
		b.state.AtHeight(header.Height).Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not retrieve available executors")
	}

	err = criteria.Validate(availableExecutors)
	if err != nil {
		return subscription.NewFailedSubscription(err, "criteria validation failed")
	}

	executionDataProvider := newExecutionDataProvider(
		b.state,
		b.headers,
		b.executionDataTracker,
		b.executionResultProvider,
		b.executionStateCache,
		criteria,
		header.Height,
	)
	accountStatusProvider := newAccountStatusProvider(b.log, executionDataProvider, statusFilter)

	return b.subscriptionFactory.Subscribe(ctx, accountStatusProvider)
}

// SubscribeAccountStatusesFromLatestBlock subscribes to the streaming of account status changes starting from a
// latest sealed block, with an optional status filter.
//
// Parameters:
// - ctx: Context for the operation.
// - filter: The account status filter used to filter account statuses.
//
// If invalid parameters will be supplied SubscribeAccountStatusesFromLatestBlock will return a failed subscription.
func (b *AccountStatusesBackend) SubscribeAccountStatusesFromLatestBlock(
	ctx context.Context,
	statusFilter state_stream.AccountStatusFilter,
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

	return b.SubscribeAccountStatusesFromStartHeight(ctx, header.Height, statusFilter, criteria)
}
