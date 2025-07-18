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
		irrecoverable.Throw(ctx, err)
		return nil, flow.BlockStatusUnknown, err
	}

	status, err := b.getBlockStatus(header)
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, flow.BlockStatusUnknown, err
	}
	return header, status, nil
}

func (b *backendBlockHeaders) GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, flow.BlockStatus, error) {
	header, err := b.headers.ByBlockID(id)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(err)
	}

	status, err := b.getBlockStatus(header)
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, flow.BlockStatusUnknown, err
	}
	return header, status, nil
}

func (b *backendBlockHeaders) GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, flow.BlockStatus, error) {
	header, err := b.headers.ByHeight(height)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(resolveHeightError(b.state.Params(), height, err))
	}

	status, err := b.getBlockStatus(header)
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, flow.BlockStatusUnknown, err
	}
	return header, status, nil
}
