package protocol

import (
	"context"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type NetworkAPI interface {
	GetNetworkParameters(ctx context.Context) access.NetworkParameters
	GetLatestProtocolStateSnapshot(ctx context.Context) ([]byte, error)
	GetProtocolStateSnapshotByBlockID(ctx context.Context, blockID flow.Identifier) ([]byte, error)
	GetProtocolStateSnapshotByHeight(ctx context.Context, blockHeight uint64) ([]byte, error)
	GetNodeVersionInfo(ctx context.Context) (*access.NodeVersionInfo, error)
}

type API interface {
	NetworkAPI
	GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, error)
	GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, error)
	GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, error)
	GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, error)
	GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, error)
	GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, error)
}

type backend struct {
	NetworkAPI
	blocks  storage.Blocks
	headers storage.Headers
	state   protocol.State
}

func New(
	state protocol.State,
	blocks storage.Blocks,
	headers storage.Headers,
	network NetworkAPI,
) API {
	return &backend{
		NetworkAPI: network,
		headers:    headers,
		blocks:     blocks,
		state:      state,
	}
}

func (b *backend) GetLatestBlock(_ context.Context, isSealed bool) (*flow.Block, error) {
	header, err := b.getLatestHeader(isSealed)
	if err != nil {
		err = rpc.ConvertStorageError(err)
		return nil, err
	}

	block, err := b.blocks.ByID(header.ID())
	if err != nil {
		err = rpc.ConvertStorageError(err)
		return nil, err
	}

	return block, nil
}

func (b *backend) GetBlockByID(_ context.Context, id flow.Identifier) (*flow.Block, error) {
	block, err := b.blocks.ByID(id)
	if err != nil {
		err = rpc.ConvertStorageError(err)
		return nil, err
	}

	return block, nil
}

func (b *backend) GetBlockByHeight(_ context.Context, height uint64) (*flow.Block, error) {
	block, err := b.blocks.ByHeight(height)
	if err != nil {
		err = rpc.ConvertStorageError(err)
		return nil, err
	}

	return block, nil
}

func (b *backend) GetLatestBlockHeader(_ context.Context, isSealed bool) (*flow.Header, error) {
	header, err := b.getLatestHeader(isSealed)
	if err != nil {
		err = rpc.ConvertStorageError(err)
		return nil, err
	}

	return header, nil
}

func (b *backend) GetBlockHeaderByID(_ context.Context, id flow.Identifier) (*flow.Header, error) {
	header, err := b.headers.ByBlockID(id)
	if err != nil {
		err = rpc.ConvertStorageError(err)
		return nil, err
	}

	return header, nil
}

func (b *backend) GetBlockHeaderByHeight(_ context.Context, height uint64) (*flow.Header, error) {
	header, err := b.headers.ByHeight(height)
	if err != nil {
		err = rpc.ConvertStorageError(err)
		return nil, err
	}

	return header, nil
}

func (b *backend) getLatestHeader(isSealed bool) (*flow.Header, error) {
	var header *flow.Header
	var err error

	if isSealed {
		// get the latest seal header from storage
		header, err = b.state.Sealed().Head()
		return header, err
	} else {
		// get the finalized header from state
		header, err = b.state.Final().Head()
		return header, err
	}
}
