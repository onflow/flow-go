package backend

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type backendBlockDetails struct {
	backendBlockBase
}

func (b *backendBlockDetails) GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, flow.BlockStatus, error) {
	var header *flow.Header
	var blockStatus flow.BlockStatus
	var err error

	if isSealed {
		header, err = b.state.Sealed().Head()
		if err != nil {
			// sealed header must exist in the db, otherwise the node's state may be corrupt
			err = irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
			irrecoverable.Throw(ctx, err)
			return nil, flow.BlockStatusUnknown, err
		}
		blockStatus = flow.BlockStatusSealed
	} else {
		header, err = b.state.Final().Head()
		if err != nil {
			// finalized header must exist in the db, otherwise the node's state may be corrupt
			err = irrecoverable.NewExceptionf("failed to lookup final header: %w", err)
			irrecoverable.Throw(ctx, err)
			return nil, flow.BlockStatusUnknown, err
		}

		// Note: there is a corner case when requesting the latest finalized block before the
		// consensus follower has progressed past the spork root block. In this case, the returned
		// blockStatus will be finalized, however, the block is actually sealed.
		if header.Height == b.state.Params().SporkRootBlockHeight() {
			blockStatus = flow.BlockStatusSealed
		} else {
			blockStatus = flow.BlockStatusFinalized
		}
	}

	// since we are querying a finalized or sealed block, we can use the height index and save an ID computation
	block, err := b.blocks.ByHeight(header.Height)
	if err != nil {
		return nil, flow.BlockStatusUnknown, status.Errorf(codes.Internal, "could not get latest block: %v", err)
	}

	return block, blockStatus, nil
}

func (b *backendBlockDetails) GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByID(id)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(err)
	}

	status, err := b.getBlockStatus(block.ToHeader())
	if err != nil {
		// Any error returned is an indication of a bug or state corruption. we must not continue processing.
		err = irrecoverable.NewException(err)
		irrecoverable.Throw(ctx, err)
		return nil, flow.BlockStatusUnknown, err
	}
	return block, status, nil
}

func (b *backendBlockDetails) GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByHeight(height)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(common.ResolveHeightError(b.state.Params(), height, err))
	}

	status, err := b.getBlockStatus(block.ToHeader())
	if err != nil {
		// Any error returned is an indication of a bug or state corruption. we must not continue processing.
		err = irrecoverable.NewException(err)
		irrecoverable.Throw(ctx, err)
		return nil, flow.BlockStatusUnknown, err
	}

	return block, status, nil
}
