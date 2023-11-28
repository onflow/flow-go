package backend

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendBlockHeaders struct {
	headers storage.Headers
	state   protocol.State
}

func (b *backendBlockHeaders) GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, flow.BlockStatus, error) {
	var header *flow.Header
	var err error

	if isSealed {
		// get the latest seal header from storage
		header, err = b.state.Sealed().Head()
		if err != nil {
			err = irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		}
	} else {
		// get the finalized header from state
		header, err = b.state.Final().Head()
		if err != nil {
			err = irrecoverable.NewExceptionf("failed to lookup final header: %w", err)
		}
	}

	if err != nil {
		// node should always have the latest block
		// In the RPC engine, if we encounter an error from the protocol state indicating state corruption,
		// we should halt processing requests, but do throw an exception which might cause a crash:
		// - It is unsafe to process requests if we have an internally bad state.
		// - We would like to avoid throwing an exception as a result of an Access API request by policy
		//   because this can cause DOS potential
		// - Since the protocol state is widely shared, we assume that in practice another component will
		//   observe the protocol state error and throw an exception.
		irrecoverable.Throw(ctx, err)
		return nil, flow.BlockStatusUnknown, status.Errorf(codes.Internal, err.Error())
	}

	stat, err := b.getBlockStatus(ctx, header)
	if err != nil {
		return nil, stat, err
	}
	return header, stat, nil
}

func (b *backendBlockHeaders) GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, flow.BlockStatus, error) {
	header, err := b.headers.ByBlockID(id)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(err)
	}

	stat, err := b.getBlockStatus(ctx, header)
	if err != nil {
		return nil, stat, err
	}
	return header, stat, nil
}

func (b *backendBlockHeaders) GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, flow.BlockStatus, error) {
	header, err := b.headers.ByHeight(height)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(err)
	}

	stat, err := b.getBlockStatus(ctx, header)
	if err != nil {
		return nil, stat, err
	}
	return header, stat, nil
}

// No errors are expected during normal operations.
func (b *backendBlockHeaders) getBlockStatus(ctx context.Context, header *flow.Header) (flow.BlockStatus, error) {
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		// In the RPC engine, if we encounter an error from the protocol state indicating state corruption,
		// we should halt processing requests, but do throw an exception which might cause a crash:
		// - It is unsafe to process requests if we have an internally bad State.
		// - We would like to avoid throwing an exception as a result of an Access API request by policy
		//   because this can cause DOS potential
		// - Since the protocol state is widely shared, we assume that in practice another component will
		//   observe the protocol state error and throw an exception.
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return flow.BlockStatusUnknown, status.Errorf(codes.Internal, err.Error())
	}

	if header.Height > sealed.Height {
		return flow.BlockStatusFinalized, nil
	}
	return flow.BlockStatusSealed, nil
}
