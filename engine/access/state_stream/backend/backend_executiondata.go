package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ExecutionDataResponse bundles the execution data returned for a single block.
type ExecutionDataResponse struct {
	Height           uint64
	ExecutionData    *execution_data.BlockExecutionData
	BlockTimestamp   time.Time
	ExecutorMetadata accessmodel.ExecutorMetadata
}

// ExecutionDataBackend exposes read-only access to execution data.
type ExecutionDataBackend struct {
	log     zerolog.Logger
	state   protocol.State
	headers storage.Headers

	subscriptionHandler  *subscription.SubscriptionHandler
	executionDataTracker tracker.ExecutionDataTracker

	executionResultProvider optimistic_sync.ExecutionResultInfoProvider
	executionStateCache     optimistic_sync.ExecutionStateCache
}

func NewExecutionDataBackend(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	subscriptionHandler *subscription.SubscriptionHandler,
	executionDataTracker tracker.ExecutionDataTracker,
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider,
	executionStateCache optimistic_sync.ExecutionStateCache,
) *ExecutionDataBackend {
	return &ExecutionDataBackend{
		log:                     log.With().Str("module", "execution_data_backend").Logger(),
		state:                   state,
		headers:                 headers,
		subscriptionHandler:     subscriptionHandler,
		executionDataTracker:    executionDataTracker,
		executionResultProvider: executionResultProvider,
		executionStateCache:     executionStateCache,
	}
}

// GetExecutionDataByBlockID retrieves execution data for a specific block by its block ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
//
// Expected errors:
// - [access.DataNotFoundError]: when data required to process the request is not available.
func (b *ExecutionDataBackend) GetExecutionDataByBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	criteria optimistic_sync.Criteria,
) (*execution_data.BlockExecutionData, *accessmodel.ExecutorMetadata, error) {
	execResultInfo, err := b.executionResultProvider.ExecutionResultInfo(blockID, criteria)
	// TODO: change error handling; it is obsolete.
	if err != nil {
		err = fmt.Errorf("failed to get execution result info for block: %w", err)
		switch {
		case errors.Is(err, storage.ErrNotFound):
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		case errors.Is(err, optimistic_sync.ErrNotEnoughAgreeingExecutors):
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		default:
			return nil, nil, access.RequireNoError(ctx, err)
		}
	}

	executionResultID := execResultInfo.ExecutionResultID
	snapshot, err := b.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = fmt.Errorf("failed to find snapshot by execution result ID %s: %w", executionResultID.String(), err)
		return nil, nil, access.NewDataNotFoundError("snapshot", err)
	}

	executionData, err := snapshot.BlockExecutionData().ByBlockID(ctx, blockID)
	if err != nil {
		// need custom not found handler due to blob not found error
		if errors.Is(err, storage.ErrNotFound) ||
			execution_data.IsBlobNotFoundError(err) {
			err = fmt.Errorf("could not find execution data for block %s: %w", blockID, err)
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		}

		// any other error is unexpected exception and indicates there is a bug or inconsistent state.
		return nil, nil, access.RequireNoError(ctx, fmt.Errorf("unexpected error getting execution data: %w", err))
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultID,
		ExecutorIDs:       execResultInfo.ExecutionNodes.NodeIDs(),
	}

	return executionData.BlockExecutionData, metadata, nil
}

// SubscribeExecutionData is deprecated and will be removed in future versions.
// Use SubscribeExecutionDataFromStartBlockID, SubscribeExecutionDataFromStartBlockHeight or SubscribeExecutionDataFromLatest.
//
// SubscribeExecutionData streams execution data for all blocks starting at the specified block ID or block height
// up until the latest available block. Once the latest is reached, the stream will remain open and responses
// are sent for each new block as it becomes available.
//
// Only one of startBlockID and startHeight may be set. If neither startBlockID nor startHeight is provided,
// the latest sealed block is used.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block. If provided, startHeight should be 0.
// - startHeight: The height of the starting block. If provided, startBlockID should be flow.ZeroID.
//
// If invalid parameters are provided, failed subscription will be returned.
func (b *ExecutionDataBackend) SubscribeExecutionData(
	ctx context.Context,
	startBlockID flow.Identifier,
	startHeight uint64,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeight(ctx, startBlockID, startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start block height")
	}

	executionDataProvider := newExecutionDataProvider(
		b.state,
		b.headers,
		b.executionResultProvider,
		b.executionStateCache,
		criteria,
		nextHeight,
	)

	return b.subscriptionHandler.Subscribe(ctx, executionDataProvider)
}

// SubscribeExecutionDataFromStartBlockID streams execution data for all blocks starting at the specified block ID
// up until the latest available block. Once the latest is reached, the stream will remain open and responses
// are sent for each new block as it becomes available.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block.
//
// If invalid parameters are provided, failed subscription will be returned.
func (b *ExecutionDataBackend) SubscribeExecutionDataFromStartBlockID(
	ctx context.Context,
	startBlockID flow.Identifier,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start block height")
	}

	executionDataProvider := newExecutionDataProvider(
		b.state,
		b.headers,
		b.executionResultProvider,
		b.executionStateCache,
		criteria,
		nextHeight,
	)

	return b.subscriptionHandler.Subscribe(ctx, executionDataProvider)
}

// SubscribeExecutionDataFromStartBlockHeight streams execution data for all blocks starting at the specified block height
// up until the latest available block. Once the latest is reached, the stream will remain open and responses
// are sent for each new block as it becomes available.
//
// Parameters:
// - ctx: Context for the operation.
// - startHeight: The height of the starting block.
//
// If invalid parameters are provided, failed subscription will be returned.
func (b *ExecutionDataBackend) SubscribeExecutionDataFromStartBlockHeight(
	ctx context.Context,
	startBlockHeight uint64,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	// TODO: can use headers, though i need a check that startBlockHeight is not greater than the highest indexed height,
	// i can pass this value to backend without having executionDataTracker
	nextHeight, err := b.executionDataTracker.GetStartHeightFromHeight(startBlockHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start block height")
	}

	executionDataProvider := newExecutionDataProvider(
		b.state,
		b.headers,
		b.executionResultProvider,
		b.executionStateCache,
		criteria,
		nextHeight,
	)

	return b.subscriptionHandler.Subscribe(ctx, executionDataProvider)
}

// SubscribeExecutionDataFromLatest streams execution data starting at the latest block.
// Once the latest is reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Parameters:
// - ctx: Context for the operation.
//
// If invalid parameters are provided, failed subscription will be returned.
func (b *ExecutionDataBackend) SubscribeExecutionDataFromLatest(
	ctx context.Context,
	criteria optimistic_sync.Criteria,
) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start block height")
	}

	executionDataProvider := newExecutionDataProvider(
		b.state,
		b.headers,
		b.executionResultProvider,
		b.executionStateCache,
		criteria,
		nextHeight,
	)

	return b.subscriptionHandler.Subscribe(ctx, executionDataProvider)
}

type executionDataProvider struct {
	state                   protocol.State
	headers                 storage.Headers
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider
	executionStateCache     optimistic_sync.ExecutionStateCache
	criteria                optimistic_sync.Criteria
	height                  uint64
}

func newExecutionDataProvider(
	state protocol.State,
	headers storage.Headers,
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider,
	executionStateCache optimistic_sync.ExecutionStateCache,
	nextCriteria optimistic_sync.Criteria,
	startHeight uint64,
) *executionDataProvider {
	return &executionDataProvider{
		state:                   state,
		headers:                 headers,
		executionResultProvider: executionResultProvider,
		executionStateCache:     executionStateCache,
		criteria:                nextCriteria,
		height:                  startHeight,
	}
}

var _ subscription.DataProvider = (*executionDataProvider)(nil)

func (e *executionDataProvider) NextData(ctx context.Context) (any, error) {
	// TODO: there was a check like if height > highestAvailableHeight { return err }
	// highest height were produced by the execution data downloader in the access_node_builder.go
	// it was passed as an argument to the execution data tracker.
	// should i add back this check here?

	// advance height for the next call
	defer func() {
		e.height += 1
	}()

	// the spork root block will never have execution data available. If requested, return an empty result.
	if e.height == e.state.Params().SporkRootBlockHeight() {
		return &ExecutionDataResponse{
			Height: e.height,
			ExecutionData: &execution_data.BlockExecutionData{
				BlockID: e.state.Params().SporkRootBlock().ID(),
			},
		}, nil
	}

	blockID, err := e.headers.BlockIDByHeight(e.height)
	if err != nil {
		// this can only happen if a block is not finalized yet. however, we don't want to serve
		// unfinalized blocks to clients, so we return an error instead.
		return nil, fmt.Errorf("block %d might not be finalized yet: %w", e.height, err)
	}

	execResultInfo, err := e.executionResultProvider.ExecutionResultInfo(blockID, e.criteria)
	if err != nil {
		switch {
		case errors.Is(err, optimistic_sync.ErrRequiredExecutorNotFound) ||
			errors.Is(err, optimistic_sync.ErrNotEnoughAgreeingExecutors):
			return nil, errors.Join(subscription.ErrBlockNotReady, err)

		case errors.Is(err, optimistic_sync.ErrBlockNotFound) ||
			errors.Is(err, optimistic_sync.ErrForkAbandoned):
			return nil, err

		default:
			return nil, fmt.Errorf("unexpected error: %w", err)
		}
	}

	executionResultID := execResultInfo.ExecutionResultID
	snapshot, err := e.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		return nil, fmt.Errorf("failed to find snapshot by execution result ID %s: %w", executionResultID.String(), err)
	}

	executionData, err := snapshot.BlockExecutionData().ByBlockID(ctx, blockID)
	if err != nil {
		return nil, fmt.Errorf("could not find execution data for block %s: %w", blockID, err)
	}

	// update criteria for the next call
	e.criteria.ParentExecutionResultID = executionResultID

	return &ExecutionDataResponse{
		Height:        e.height,
		ExecutionData: executionData.BlockExecutionData,
		ExecutorMetadata: accessmodel.ExecutorMetadata{
			ExecutionResultID: executionResultID,
			ExecutorIDs:       execResultInfo.ExecutionNodes.NodeIDs(),
		},
	}, nil
}
