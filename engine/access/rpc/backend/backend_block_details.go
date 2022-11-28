package backend

import (
	"context"
	"fmt"

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
		err = convertStorageError(err)
		return nil, flow.BlockStatusUnknown, err
	}

	block, err := b.blocks.ByID(header.ID())
	if err != nil {
		err = convertStorageError(err)
		return nil, flow.BlockStatusUnknown, err
	}

	status, err := b.getBlockStatus(block)
	if err != nil {
		return nil, status, err
	}
	return block, status, nil
}

func (b *backendBlockDetails) GetBlockByID(_ context.Context, id flow.Identifier) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByID(id)
	if err != nil {
		err = convertStorageError(err)
		return nil, flow.BlockStatusUnknown, err
	}

	status, err := b.getBlockStatus(block)
	if err != nil {
		return nil, status, err
	}
	return block, status, nil
}

func (b *backendBlockDetails) GetBlockByHeight(_ context.Context, height uint64) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByHeight(height)
	if err != nil {
		err = convertStorageError(err)
		return nil, flow.BlockStatusUnknown, err
	}

	status, err := b.getBlockStatus(block)
	if err != nil {
		return nil, status, err
	}
	return block, status, nil
}

func (b *backendBlockDetails) getBlockStatus(block *flow.Block) (flow.BlockStatus, error) {
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		return flow.BlockStatusUnknown, fmt.Errorf("failed to find latest sealed header: %w", err)
	}

	if block.Header.Height > sealed.Height {
		return flow.BlockStatusFinalized, nil
	}
	return flow.BlockStatusSealed, nil
}
