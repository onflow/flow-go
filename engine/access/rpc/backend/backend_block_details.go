package backend

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type backendBlockDetails struct {
	backendBlockBase
}

func (b *backendBlockDetails) GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, flow.BlockStatus, error) {
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

		irrecoverable.Throw(ctx, err)
		return nil, flow.BlockStatusUnknown, err
	}

	// since we are querying a finalized or sealed block, we can use the height index and save an ID computation
	block, err := b.blocks.ByHeight(header.Height)
	if err != nil {
		return nil, flow.BlockStatusUnknown, status.Errorf(codes.Internal, "could not get latest block: %v", err)
	}

	status, err := b.getBlockStatus(block.Header)
	if err != nil {
		irrecoverableErr := irrecoverable.NewException(err)
		irrecoverable.Throw(ctx, irrecoverableErr)
		return nil, flow.BlockStatusUnknown, irrecoverableErr
	}
	return block, status, nil
}

func (b *backendBlockDetails) GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByID(id)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(err)
	}

	status, err := b.getBlockStatus(block.Header)
	if err != nil {
		irrecoverableErr := irrecoverable.NewException(err)
		irrecoverable.Throw(ctx, irrecoverableErr)
		return nil, flow.BlockStatusUnknown, irrecoverableErr
	}
	return block, status, nil
}

func (b *backendBlockDetails) GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByHeight(height)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(resolveHeightError(b.state.Params(), height, err))
	}

	status, err := b.getBlockStatus(block.Header)
	if err != nil {
		irrecoverableErr := irrecoverable.NewException(err)
		irrecoverable.Throw(ctx, irrecoverableErr)
		return nil, flow.BlockStatusUnknown, irrecoverableErr
	}
	return block, status, nil
}
