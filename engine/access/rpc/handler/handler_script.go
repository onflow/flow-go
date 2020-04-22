package handler

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow/protobuf/go/flow/access"
	"github.com/dapperlabs/flow/protobuf/go/flow/execution"
)

func (h *Handler) ExecuteScriptAtLatestBlock(ctx context.Context,
	req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {

	// get the latest sealed header
	latestHeader, err := h.getLatestSealedHeader()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get latest sealed header: %v", err)
	}

	// get the block id of the latest sealed header
	latestBlockID := latestHeader.ID()

	// execute script on the execution node at that block id
	return h.executeScriptOnExecutionNode(ctx, latestBlockID[:], req.Script)
}

func (h *Handler) ExecuteScriptAtBlockID(ctx context.Context,
	req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {

	blockID := req.GetBlockId()

	// validate request
	if blockID == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid block id")
	}

	// execute script on the execution node at that block id
	return h.executeScriptOnExecutionNode(ctx, blockID, req.Script)
}

func (h *Handler) ExecuteScriptAtBlockHeight(ctx context.Context,
	req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {

	height := req.GetBlockHeight()

	// get header at given height
	header, err := h.headers.ByNumber(height)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	blockID := header.ID()

	// execute script on the execution node at that block id
	return h.executeScriptOnExecutionNode(ctx, blockID[:], req.Script)
}

// executeScriptOnExecutionNode forwards the request to the execution node using the execution node
// grpc client and converts the response back to the access node api response format
func (h *Handler) executeScriptOnExecutionNode(ctx context.Context,
	blockID []byte,
	script []byte) (*access.ExecuteScriptResponse, error) {

	executionReq := execution.ExecuteScriptAtBlockIDRequest{
		BlockId: blockID[:],
		Script:  script,
	}

	executionResp, err := h.executionRPC.ExecuteScriptAtBlockID(ctx, &executionReq)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to execute the script on the execution node: %v", err)
	}

	resp := access.ExecuteScriptResponse{
		Value: executionResp.Value,
	}

	return &resp, nil
}
