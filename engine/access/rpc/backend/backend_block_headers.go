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
		// node should always have the latest block
		return nil, flow.BlockStatusUnknown, status.Errorf(codes.Internal, "could not get latest block header: %v", err)
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
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(err)
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
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(err)
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
		return flow.BlockStatusUnknown, status.Errorf(codes.Internal, "failed to find latest sealed header: %v", err)
	}

	if header.Height > sealed.Height {
		return flow.BlockStatusFinalized, nil
	}
	return flow.BlockStatusSealed, nil
}
