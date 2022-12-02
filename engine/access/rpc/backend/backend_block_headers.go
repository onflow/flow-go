package backend

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendBlockHeaders struct {
	headers storage.Headers
	state   protocol.State
}

func (b *backendBlockHeaders) GetLatestBlockHeader(_ context.Context, isSealed bool) (*flow.Header, flow.BlockStatus, error) {
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

	status, err := b.getBlockStatus(header)
	if err != nil {
		return nil, status, err
	}
	return header, status, nil
}

func (b *backendBlockHeaders) GetBlockHeaderByID(_ context.Context, id flow.Identifier) (*flow.Header, flow.BlockStatus, error) {
	header, err := b.headers.ByBlockID(id)
	if err != nil {
		err = convertStorageError(err)
		return nil, flow.BlockStatusUnknown, err
	}

	status, err := b.getBlockStatus(header)
	if err != nil {
		return nil, status, err
	}
	return header, status, nil
}

func (b *backendBlockHeaders) GetBlockHeaderByHeight(_ context.Context, height uint64) (*flow.Header, flow.BlockStatus, error) {
	header, err := b.headers.ByHeight(height)
	if err != nil {
		err = convertStorageError(err)
		return nil, flow.BlockStatusUnknown, err
	}

	status, err := b.getBlockStatus(header)
	if err != nil {
		return nil, status, err
	}
	return header, status, nil
}

func (b *backendBlockHeaders) getBlockStatus(header *flow.Header) (flow.BlockStatus, error) {
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		return flow.BlockStatusUnknown, fmt.Errorf("failed to find latest sealed header: %w", err)
	}

	if header.Height > sealed.Height {
		return flow.BlockStatusFinalized, nil
	}
	return flow.BlockStatusSealed, nil
}
