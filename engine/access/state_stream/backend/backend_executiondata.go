package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

type ExecutionDataResponse struct {
	Height         uint64
	ExecutionData  *execution_data.BlockExecutionData
	BlockTimestamp time.Time
}

type ExecutionDataBackend struct {
	log     zerolog.Logger
	headers storage.Headers

	getExecutionData GetExecutionDataFunc

	subscriptionHandler  *subscription.SubscriptionHandler
	executionDataTracker subscription.ExecutionDataTracker
}

func (b *ExecutionDataBackend) GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionData, error) {
	header, err := b.headers.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get block header for %s: %w", blockID, err)
	}

	executionData, err := b.getExecutionData(ctx, header.Height)

	if err != nil {
		// need custom not found handler due to blob not found error
		if errors.Is(err, storage.ErrNotFound) || execution_data.IsBlobNotFoundError(err) {
			return nil, status.Errorf(codes.NotFound, "could not find execution data: %v", err)
		}

		return nil, rpc.ConvertError(err, "could not get execution data", codes.Internal)
	}

	return executionData.BlockExecutionData, nil
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
func (b *ExecutionDataBackend) SubscribeExecutionData(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeight(ctx, startBlockID, startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start block height")
	}

	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getResponse)
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
func (b *ExecutionDataBackend) SubscribeExecutionDataFromStartBlockID(ctx context.Context, startBlockID flow.Identifier) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start block height")
	}

	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getResponse)
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
func (b *ExecutionDataBackend) SubscribeExecutionDataFromStartBlockHeight(ctx context.Context, startBlockHeight uint64) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromHeight(startBlockHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start block height")
	}

	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getResponse)
}

// SubscribeExecutionDataFromLatest streams execution data starting at the latest block.
// Once the latest is reached, the stream will remain open and responses are sent for each new
// block as it becomes available.
//
// Parameters:
// - ctx: Context for the operation.
//
// If invalid parameters are provided, failed subscription will be returned.
func (b *ExecutionDataBackend) SubscribeExecutionDataFromLatest(ctx context.Context) subscription.Subscription {
	nextHeight, err := b.executionDataTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start block height")
	}

	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getResponse)
}

func (b *ExecutionDataBackend) getResponse(ctx context.Context, height uint64) (interface{}, error) {
	executionData, err := b.getExecutionData(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
	}

	return &ExecutionDataResponse{
		Height:        height,
		ExecutionData: executionData.BlockExecutionData,
	}, nil
}
