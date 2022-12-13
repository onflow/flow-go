package state_stream

import (
	"context"

	access "github.com/onflow/flow/protobuf/go/flow/executiondata"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
)

type Handler struct {
	api   API
	chain flow.Chain
}

// HandlerOption is used to hand over optional constructor parameters
type HandlerOption func(*Handler)

func NewHandler(api API, chain flow.Chain, options ...HandlerOption) *Handler {
	h := &Handler{
		api:   api,
		chain: chain,
	}
	for _, opt := range options {
		opt(h)
	}
	return h
}

func (h *Handler) GetExecutionDataByBlockID(ctx context.Context, request *access.GetExecutionDataByBlockIDRequest) (*access.GetExecutionDataByBlockIDResponse, error) {
	blockID, err := convert.BlockID(request.GetBlockId())
	if err != nil {
		return nil, err
	}

	execData, err := h.api.GetExecutionDataByBlockID(ctx, blockID)
	if err != nil {
		return nil, err
	}

	return &access.GetExecutionDataByBlockIDResponse{BlockExecutionData: execData}, nil
}

func (h *Handler) SubscribeExecutionData(request *access.SubscribeExecutionDataRequest, stream access.ExecutionDataAPI_SubscribeExecutionDataServer) error {
	ctx := stream.Context()
	sub := h.api.SubscribeExecutionData(ctx)

	for {
		// TODO: this should handle graceful shutdown from the server
		resp, ok := <-sub.Channel()
		if !ok {
			return sub.Err()
		}

		err := stream.Send(resp)
		if err != nil {
			return err
		}
	}
}
