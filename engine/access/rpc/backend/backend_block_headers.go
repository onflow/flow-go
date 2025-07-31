package backend

import (
	"context"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type backendBlockHeaders struct {
	backendBlockBase
}

func (b *backendBlockHeaders) GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.UnsignedHeader, flow.BlockStatus, error) {
	if isSealed {
		header, err := b.state.Sealed().Head()
		if err != nil {
			// sealed header must exist in the db, otherwise the node's state may be corrupt
			err = irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
			irrecoverable.Throw(ctx, err)
			return nil, flow.BlockStatusUnknown, err
		}
		return header, flow.BlockStatusSealed, nil
	}

	header, err := b.state.Final().Head()
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
		return header, flow.BlockStatusSealed, nil
	} else {
		return header, flow.BlockStatusFinalized, nil
	}
}

func (b *backendBlockHeaders) GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.UnsignedHeader, flow.BlockStatus, error) {
	header, err := b.headers.ByBlockID(id)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(err)
	}

	status, err := b.getBlockStatus(header)
	if err != nil {
		// Any error returned is an indication of a bug or state corruption. we must not continue processing.
		err = irrecoverable.NewException(err)
		irrecoverable.Throw(ctx, err)
		return nil, flow.BlockStatusUnknown, err
	}
	return header, status, nil
}

func (b *backendBlockHeaders) GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.UnsignedHeader, flow.BlockStatus, error) {
	header, err := b.headers.ByHeight(height)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(resolveHeightError(b.state.Params(), height, err))
	}

	status, err := b.getBlockStatus(header)
	if err != nil {
		// Any error returned is an indication of a bug or state corruption. we must not continue processing.
		err = irrecoverable.NewException(err)
		irrecoverable.Throw(ctx, err)
		return nil, flow.BlockStatusUnknown, err
	}
	return header, status, nil
}
