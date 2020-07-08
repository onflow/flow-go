package handler

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/dapperlabs/flow-go/engine/common/rpc/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

type handlerBlockDetails struct {
	blocks storage.Blocks
	state  protocol.State
}

func (h *handlerBlockDetails) GetLatestBlock(_ context.Context, req *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	var header *flow.Header
	var err error
	if req.IsSealed {
		// get the latest seal header from storage
		header, err = h.state.Sealed().Head()
	} else {
		// get the finalized header from state
		header, err = h.state.Final().Head()
	}

	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	block, err := h.blocks.ByID(header.ID())
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return createBlockResponse(block)
}

func (h *handlerBlockDetails) GetBlockByID(_ context.Context, req *access.GetBlockByIDRequest) (*access.BlockResponse, error) {

	id := flow.HashToID(req.Id)
	block, err := h.blocks.ByID(id)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return createBlockResponse(block)
}

func (h *handlerBlockDetails) GetBlockByHeight(_ context.Context, req *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {

	block, err := h.blocks.ByHeight(req.Height)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return createBlockResponse(block)
}

func createBlockResponse(block *flow.Block) (*access.BlockResponse, error) {
	msg, err := convert.BlockToMessage(block)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	resp := &access.BlockResponse{
		Block: msg,
	}
	return resp, nil
}
