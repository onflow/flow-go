package backend

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendBlockDetails struct {
	blocks storage.Blocks
	state  protocol.State
}

func (b *backendBlockDetails) GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, flow.BlockStatus, error) {
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
		err = convertStorageError(err)
		return nil, flow.BlockStatusUnknown, err
	}

	block, err := b.blocks.ByID(header.ID())
	if err != nil {
		err = convertStorageError(err)
		return nil, flow.BlockStatusUnknown, err
	}
	status := b.getBlockStatus(ctx, block)
	return block, status, nil
}

func (b *backendBlockDetails) GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByID(id)
	if err != nil {
		err = convertStorageError(err)
		return nil, flow.BlockStatusUnknown, err
	}
	status := b.getBlockStatus(ctx, block)
	return block, status, nil
}

func (b *backendBlockDetails) GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByHeight(height)
	if err != nil {
		err = convertStorageError(err)
		return nil, flow.BlockStatusUnknown, err
	}
	status := b.getBlockStatus(ctx, block)
	return block, status, nil
}

func (b *backendBlockDetails) getBlockStatus(_ context.Context, block *flow.Block) flow.BlockStatus {
	latest, err := b.state.Sealed().Head()
	if err != nil {
		return flow.BlockStatusUnknown
	}

	if block.Header.Height > latest.Height {
		return flow.BlockStatusFinalized
	}
	return flow.BlockStatusSealed
}
