package handler

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/execution"
)

// GetEventsForHeightRange retrieves events for all sealed blocks between the start block height and the end block height (inclusive) that have the given type
func (h *Handler) GetEventsForHeightRange(ctx context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {

	// validate the request
	if req.GetEndHeight() < req.GetStartHeight() {
		return nil, status.Error(codes.InvalidArgument, "invalid start or end height")
	}

	reqEvent := req.GetType()
	if reqEvent == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid event type")
	}

	// get the latest sealed block header
	head, err := h.state.Sealed().Head()
	if err != nil {
		return nil, status.Errorf(codes.Internal, " failed to get events: %v", err)
	}

	var minHeight, maxHeight uint64

	// derive bounds for block height
	minHeight = req.GetStartHeight()

	// limit max height to last sealed block in the chain
	if head.Height < req.GetEndHeight() {
		maxHeight = head.Height
	} else {
		maxHeight = req.GetEndHeight()
	}

	// find the block IDs for all the blocks between min and max height (inclusive)
	blockIDs := make([][]byte, 0)
	for i := minHeight; i <= maxHeight; i++ {
		block, err := h.blocks.ByHeight(i)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get events: %v", err)
		}
		id := block.ID()
		blockIDs = append(blockIDs, id[:])
	}

	return h.getBlockEventsFromExecutionNode(ctx, blockIDs, reqEvent)
}

// GetEventsForBlockIDs retrieves events for all the specified block IDs that have the given type
func (h *Handler) GetEventsForBlockIDs(ctx context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {

	if len(req.GetBlockIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "block IDs not specified")
	}

	if req.GetType() == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid event type")
	}

	// forward the request to the execution node
	return h.getBlockEventsFromExecutionNode(ctx, req.GetBlockIds(), req.GetType())
}

func (h *Handler) getBlockEventsFromExecutionNode(ctx context.Context, blockIDs [][]byte, etype string) (*access.EventsResponse, error) {

	// create an execution API request for events at block ID
	req := execution.GetEventsForBlockIDsRequest{
		Type:     etype,
		BlockIds: blockIDs,
	}

	// call the execution node GRPC
	resp, err := h.executionRPC.GetEventsForBlockIDs(ctx, &req)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve events from execution node: %v", err)
	}

	// convert execution node api result to access node api result
	results := accessEvents(resp.GetResults())

	return &access.EventsResponse{
		Results: results,
	}, nil
}

func (h *Handler) getTransactionResultFromExecutionNode(ctx context.Context, blockID []byte, transactionID []byte) ([]*entities.Event, uint32, string, error) {

	// create an execution API request for events at blockID and transactionID
	req := execution.GetTransactionResultRequest{
		BlockId:       blockID,
		TransactionId: transactionID,
	}

	// call the execution node GRPC
	resp, err := h.executionRPC.GetTransactionResult(ctx, &req)

	if err != nil {
		return nil, 0, "", status.Errorf(codes.Internal, "failed to retrieve result from execution node: %v", err)
	}

	exeResults := resp.GetEvents()

	return exeResults, resp.GetStatusCode(), resp.GetErrorMessage(), nil
}

// accessEvents converts execution node api result to access node api result
func accessEvents(execEvents []*execution.GetEventsForBlockIDsResponse_Result) []*access.EventsResponse_Result {

	results := make([]*access.EventsResponse_Result, len(execEvents))

	for i, r := range execEvents {
		results[i] = &access.EventsResponse_Result{
			BlockId:     r.GetBlockId(),
			BlockHeight: r.GetBlockHeight(),
			Events:      r.GetEvents(),
		}
	}

	return results
}
