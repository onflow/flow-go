package state_stream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
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
	log         zerolog.Logger
	headers     storage.Headers
	broadcaster *engine.Broadcaster
	sendTimeout time.Duration

	getExecutionData GetExecutionDataFunc
	getStartHeight   GetStartHeightFunc
}

func (b *ExecutionDataBackend) GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionData, error) {
	executionData, err := b.getExecutionData(ctx, blockID)

	// need custom not found handler due to blob not found error
	if errors.Is(err, storage.ErrNotFound) || execution_data.IsBlobNotFoundError(err) {
		return nil, status.Errorf(codes.NotFound, "could not find execution data: %v", err)
	}

	if err != nil {
		return nil, rpc.ConvertError(err, "could not get execution data", codes.Internal)
	}

	return executionData.BlockExecutionData, nil
}

func (b *ExecutionDataBackend) SubscribeExecutionData(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) Subscription {
	sub := NewHeightBasedSubscription(b.getResponse)

	nextHeight, err := b.getStartHeight(startBlockID, startHeight)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			sub.Fail(status.Errorf(st.Code(), "could not get start height: %v", st.Message()))
			return sub
		}

		sub.Fail(fmt.Errorf("could not get start height: %w", err))
		return sub
	}

	sub.nextHeight = nextHeight

	go NewStreamer(b.log, b.broadcaster, b.sendTimeout, sub).Stream(ctx)

	return sub
}

func (b *ExecutionDataBackend) getResponse(ctx context.Context, height uint64) (interface{}, error) {
	header, err := b.headers.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get block header for height %d: %w", height, err)
	}

	executionData, err := b.getExecutionData(ctx, header.ID())
	if err != nil {
		return nil, err
	}

	return &ExecutionDataResponse{
		Height:        header.Height,
		ExecutionData: executionData.BlockExecutionData,
	}, nil
}
