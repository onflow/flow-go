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

type handlerEvents struct {
	executionRPC execution.ExecutionAPIClient
	blocks       storage.Blocks
	state        protocol.State
}

// GetEventsForHeightRange retrieves events for all sealed blocks between the start block height and the end block height (inclusive) that have the given type
func (h *handlerEvents) GetEventsForHeightRange(ctx context.Context, req *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {

	// validate the request
	if req.GetEndHeight() < req.GetStartHeight() {
		return nil, status.Error(codes.InvalidArgument, "invalid start or end height")
	}

	// validate the event type
	reqEvent := req.GetType()
	if err := validate.EventType(reqEvent); err != nil {
		return nil, err
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

	if err := validate.BlockIDs(blockIDs); err != nil {
		return nil, err
	}

	return h.getBlockEventsFromExecutionNode(ctx, blockIDs, reqEvent)
}

// GetEventsForBlockIDs retrieves events for all the specified block IDs that have the given type
func (h *handlerEvents) GetEventsForBlockIDs(ctx context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {

	// validate the block ids
	blockIDs := req.GetBlockIds()
	if err := validate.BlockIDs(blockIDs); err != nil {
		return nil, err
	}

	// validate the event type
	reqEvent := req.GetType()
	if err := validate.EventType(reqEvent); err != nil {
		return nil, err
	}

	// forward the request to the execution node
	return h.getBlockEventsFromExecutionNode(ctx, req.GetBlockIds(), reqEvent)
}

func (h *handlerEvents) getBlockEventsFromExecutionNode(ctx context.Context, blockIDs [][]byte, etype string) (*access.EventsResponse, error) {

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
