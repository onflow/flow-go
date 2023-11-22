package backend

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendBlockDetails struct {
	blocks storage.Blocks
	state  protocol.State
}

func (b *backendBlockDetails) GetLatestBlock(_ context.Context, isSealed bool) (*flow.Block, flow.BlockStatus, error) {
	var header *flow.Header
	var err error

	if isSealed {
		// get the latest seal header from storage
		header, err = b.state.Sealed().Head()
	} else {
		// get the finalized header from state
		header, err = b.state.Final().Head()
	}

	if err != nil {
		// node should always have the latest block

		// In the RPC engine, if we encounter an error from the protocol state indicating state corruption,
		// we should halt processing requests, but do throw an exception which might cause a crash:
		// - It is unsafe to process requests if we have an internally bad state.
		//   TODO: https://github.com/onflow/flow-go/issues/4028
		// - We would like to avoid throwing an exception as a result of an Access API request by policy
		//   because this can cause DOS potential
		// - Since the protocol state is widely shared, we assume that in practice another component will
		//   observe the protocol state error and throw an exception.
		return nil, flow.BlockStatusUnknown, status.Errorf(codes.Internal, "could not get latest block: %v", err)
	}

	// since we are querying a finalized or sealed block, we can use the height index and save an ID computation
	block, err := b.blocks.ByHeight(header.Height)
	if err != nil {
		return nil, flow.BlockStatusUnknown, status.Errorf(codes.Internal, "could not get latest block: %v", err)
	}

	stat, err := b.getBlockStatus(block)
	if err != nil {
		return nil, stat, err
	}
	return block, stat, nil
}

func (b *backendBlockDetails) GetBlockByID(_ context.Context, id flow.Identifier) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByID(id)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(err)
	}

	stat, err := b.getBlockStatus(block)
	if err != nil {
		return nil, stat, err
	}
	return block, stat, nil
}

func (b *backendBlockDetails) GetBlockByHeight(_ context.Context, height uint64) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByHeight(height)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(err)
	}

	stat, err := b.getBlockStatus(block)
	if err != nil {
		return nil, stat, err
	}
	return block, stat, nil
}

func (b *backendBlockDetails) getBlockStatus(block *flow.Block) (flow.BlockStatus, error) {
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		// In the RPC engine, if we encounter an error from the protocol state indicating state corruption,
		// we should halt processing requests, but do throw an exception which might cause a crash:
		// - It is unsafe to process requests if we have an internally bad state.
		//   TODO: https://github.com/onflow/flow-go/issues/4028
		// - We would like to avoid throwing an exception as a result of an Access API request by policy
		//   because this can cause DOS potential
		// - Since the protocol state is widely shared, we assume that in practice another component will
		//   observe the protocol state error and throw an exception.
		return flow.BlockStatusUnknown, status.Errorf(codes.Internal, "failed to find latest sealed header: %v", err)
	}

	if block.Header.Height > sealed.Height {
		return flow.BlockStatusFinalized, nil
	}
	return flow.BlockStatusSealed, nil
}
