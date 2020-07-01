package handler

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/dapperlabs/flow-go/engine/common/rpc/validate"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

type handlerScripts struct {
	headers      storage.Headers
	state        protocol.State
	executionRPC execution.ExecutionAPIClient
}

func (h *handlerScripts) ExecuteScriptAtLatestBlock(ctx context.Context,
	req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {

	// get the latest sealed header
	latestHeader, err := h.state.Sealed().Head()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get latest sealed header: %v", err)
	}

	// get the block id of the latest sealed header
	latestBlockID := latestHeader.ID()

	// execute script on the execution node at that block id
	return h.executeScriptOnExecutionNode(ctx, latestBlockID[:], req.Script)
}

func (h *handlerScripts) ExecuteScriptAtBlockID(ctx context.Context,
	req *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {

	blockID := req.GetBlockId()

	// validate request
	if err := validate.BlockID(blockID); err != nil {
		return nil, err
	}

	// execute script on the execution node at that block id
	return h.executeScriptOnExecutionNode(ctx, blockID, req.Script)
}

func (h *handlerScripts) ExecuteScriptAtBlockHeight(ctx context.Context,
	req *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {

	height := req.GetBlockHeight()

	// get header at given height
	header, err := h.headers.ByHeight(height)
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
func (h *handlerScripts) executeScriptOnExecutionNode(ctx context.Context,
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
