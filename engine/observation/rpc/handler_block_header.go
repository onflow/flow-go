package rpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
)

func (h *Handler) GetLatestBlockHeader(ctx context.Context, req *observation.GetLatestBlockHeaderRequest) (*observation.BlockHeaderResponse, error) {

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

	return createBlockHeaderResponse(header)
}

func (h *Handler) GetBlockHeaderByID(_ context.Context, req *observation.GetBlockHeaderByIDRequest) (*observation.BlockHeaderResponse, error) {

	id := flow.HashToID(req.Id)
	header, err := h.headers.ByBlockID(id)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return createBlockHeaderResponse(header)
}

func (h *Handler) GetBlockHeaderByHeight(_ context.Context, req *observation.GetBlockHeaderByHeightRequest) (*observation.BlockHeaderResponse, error) {

	header, err := h.headers.ByNumber(req.Height)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return createBlockHeaderResponse(header)
}

func createBlockHeaderResponse(header *flow.Header) (*observation.BlockHeaderResponse, error) {
	msg, err := convert.BlockHeaderToMessage(header)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not convert block header: %s", err.Error())
	}

	resp := &observation.BlockHeaderResponse{
		Block: &msg,
	}
	return resp, nil
}
