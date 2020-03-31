package rpc

import (
	"context"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
)

func (h *Handler) GetLatestBlock(_ context.Context, req *observation.GetLatestBlockRequest) (*observation.BlockResponse, error) {
	var header *flow.Header
	var err error
	if req.IsSealed {
		// get the latest seal header from storage
		header, err = h.getLatestSealedHeader()
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

func (h *Handler) GetBlockByID(_ context.Context, req *observation.GetBlockByIDRequest) (*observation.BlockResponse, error) {

	id := flow.HashToID(req.Id)
	block, err := h.blocks.ByID(id)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return createBlockResponse(block)
}

func (h *Handler) GetBlockByHeight(_ context.Context, req *observation.GetBlockByHeightRequest) (*observation.BlockResponse, error) {

	block, err := h.blocks.ByHeight(req.Height)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return createBlockResponse(block)
}

func createBlockResponse(block *flow.Block) (*observation.BlockResponse, error) {
	msg, err := convert.BlockToMessage(block)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	resp := &observation.BlockResponse{
		Block: msg,
	}
	return resp, nil
}
