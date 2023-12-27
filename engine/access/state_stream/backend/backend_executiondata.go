package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

type ExecutionDataResponse struct {
	Height        uint64
	ExecutionData *execution_data.BlockExecutionData
}

type ExecutionDataBackend struct {
	log            zerolog.Logger
	headers        storage.Headers
	broadcaster    *engine.Broadcaster
	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int

	getExecutionData GetExecutionDataFunc
	getStartHeight   subscription.GetStartHeightFunc
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

func (b *ExecutionDataBackend) SubscribeExecutionData(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) subscription.Subscription {
	nextHeight, err := b.getStartHeight(startBlockID, startHeight, flow.BlockStatusFinalized)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	sub := subscription.NewHeightBasedSubscription(b.sendBufferSize, nextHeight, b.getResponse)

	go subscription.NewStreamer(b.log, b.broadcaster, b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
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
