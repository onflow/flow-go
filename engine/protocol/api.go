package protocol

import (
	"context"
	"errors"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type API interface {
	GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, error)
	GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, error)
	GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, error)
	GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, error)
	GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, error)
	GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, error)
}

type backend struct {
	blocks  storage.Blocks
	headers storage.Headers
	state   protocol.State
}

func New(
	state protocol.State,
	blocks storage.Blocks,
	headers storage.Headers,
) API {
	b := &backend{

		headers: headers,
		blocks:  blocks,
		state:   state,
	}
	return b
}

func (b *backend) GetLatestBlock(_ context.Context, isSealed bool) (*flow.Block, error) {
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
		return nil, err
	}

	block, err := b.blocks.ByID(header.ID())
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return block, nil
}

func (b *backend) GetBlockByID(_ context.Context, id flow.Identifier) (*flow.Block, error) {
	block, err := b.blocks.ByID(id)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return block, nil
}

func (b *backend) GetBlockByHeight(_ context.Context, height uint64) (*flow.Block, error) {
	block, err := b.blocks.ByHeight(height)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return block, nil
}

func (b *backend) GetLatestBlockHeader(_ context.Context, isSealed bool) (*flow.Header, error) {
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
		return nil, err
	}

	return header, nil
}

func (b *backend) GetBlockHeaderByID(_ context.Context, id flow.Identifier) (*flow.Header, error) {
	header, err := b.headers.ByBlockID(id)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return header, nil
}

func (b *backend) GetBlockHeaderByHeight(_ context.Context, height uint64) (*flow.Header, error) {
	header, err := b.headers.ByHeight(height)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return header, nil
}

func convertStorageError(err error) error {
	if err == nil {
		return nil
	}
	if status.Code(err) == codes.NotFound {
		// Already converted
		return err
	}
	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "not found: %v", err)
	}

	return status.Errorf(codes.Internal, "failed to find: %v", err)
}
