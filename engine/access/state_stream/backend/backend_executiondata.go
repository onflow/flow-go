package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/height_source"
	"github.com/onflow/flow-go/engine/access/subscription/streamer"
	subimpl "github.com/onflow/flow-go/engine/access/subscription/subscription"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/storage"
)

// ExecutionDataBackend exposes read-only access to execution data.
type ExecutionDataBackend struct {
	log     zerolog.Logger
	headers storage.Headers

	getExecutionData GetExecutionDataFunc

	executionDataTracker     subscription.ExecutionDataTracker
	executionDataBroadcaster *engine.Broadcaster
	streamOptions            *streamer.StreamOptions
	endHeight                uint64

	executionResultProvider optimistic_sync.ExecutionResultInfoProvider
	executionStateCache     optimistic_sync.ExecutionStateCache
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
	if err != nil {
		err = fmt.Errorf("failed to get execution result info for block: %w", err)
		switch {
		case errors.Is(err, storage.ErrNotFound):
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		case common.IsInsufficientExecutionReceipts(err):
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
) subscription.Subscription[*state_stream.ExecutionDataResponse] {
	startHeight, err := b.executionDataTracker.GetStartHeight(ctx, startBlockID, startHeight)
	if err != nil {
		return subimpl.NewFailedSubscription[*state_stream.ExecutionDataResponse](err, "could not get start block height")
	}

	heightSource := height_source.NewHeightSource(
		startHeight,
		b.endHeight,
		b.readyUpToHeight,
		b.getExecutionDataAtHeight,
	)

	sub := subimpl.NewSubscription[*state_stream.ExecutionDataResponse](b.streamOptions.SendBufferSize)
	streamer := streamer.NewHeightBasedStreamer(
		b.log,
		b.executionDataBroadcaster,
		sub,
		heightSource,
		b.streamOptions,
	)
	go streamer.Stream(ctx)

	return sub
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
) subscription.Subscription[*state_stream.ExecutionDataResponse] {
	startHeight, err := b.executionDataTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return subimpl.NewFailedSubscription[*state_stream.ExecutionDataResponse](err, "could not get start block height")
	}

	heightSource := height_source.NewHeightSource(
		startHeight,
		b.endHeight,
		b.readyUpToHeight,
		b.getExecutionDataAtHeight,
	)

	sub := subimpl.NewSubscription[*state_stream.ExecutionDataResponse](b.streamOptions.SendBufferSize)
	streamer := streamer.NewHeightBasedStreamer(
		b.log,
		b.executionDataBroadcaster,
		sub,
		heightSource,
		b.streamOptions,
	)
	go streamer.Stream(ctx)

	return sub
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
) subscription.Subscription[*state_stream.ExecutionDataResponse] {
	startHeight, err := b.executionDataTracker.GetStartHeightFromHeight(startBlockHeight)
	if err != nil {
		return subimpl.NewFailedSubscription[*state_stream.ExecutionDataResponse](err, "could not get start block height")
	}

	heightSource := height_source.NewHeightSource(
		startHeight,
		b.endHeight,
		b.readyUpToHeight,
		b.getExecutionDataAtHeight,
	)

	sub := subimpl.NewSubscription[*state_stream.ExecutionDataResponse](b.streamOptions.SendBufferSize)
	streamer := streamer.NewHeightBasedStreamer(
		b.log,
		b.executionDataBroadcaster,
		sub,
		heightSource,
		b.streamOptions,
	)
	go streamer.Stream(ctx)

	return sub
}

// SubscribeExecutionDataFromLatest streams execution data starting at the latest block.
// Once the latest is reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Parameters:
// - ctx: Context for the operation.
//
// If invalid parameters are provided, failed subscription will be returned.
func (b *ExecutionDataBackend) SubscribeExecutionDataFromLatest(ctx context.Context) subscription.Subscription[*state_stream.ExecutionDataResponse] {
	startHeight, err := b.executionDataTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return subimpl.NewFailedSubscription[*state_stream.ExecutionDataResponse](err, "could not get start block height")
	}

	heightSource := height_source.NewHeightSource(
		startHeight,
		b.endHeight,
		b.readyUpToHeight,
		b.getExecutionDataAtHeight,
	)

	sub := subimpl.NewSubscription[*state_stream.ExecutionDataResponse](b.streamOptions.SendBufferSize)
	streamer := streamer.NewHeightBasedStreamer(
		b.log,
		b.executionDataBroadcaster,
		sub,
		heightSource,
		b.streamOptions,
	)
	go streamer.Stream(ctx)

	return sub
}

func (b *ExecutionDataBackend) getExecutionDataAtHeight(ctx context.Context, height uint64) (*state_stream.ExecutionDataResponse, error) {
	executionData, err := b.getExecutionData(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
	}

	return &state_stream.ExecutionDataResponse{
		Height:        height,
		ExecutionData: executionData.BlockExecutionData,
	}, nil
}

func (b *ExecutionDataBackend) readyUpToHeight() (uint64, error) {
	return b.executionDataTracker.GetHighestHeight(), nil
}
