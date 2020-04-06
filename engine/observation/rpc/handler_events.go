package rpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	access "github.com/dapperlabs/flow-go/protobuf/services/access"
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
	head, err := h.getLatestSealedHeader()
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
			return nil, status.Errorf(codes.Internal, " failed to get events: %v", err)
		}
		id := block.ID()
		blockIDs = append(blockIDs, id[:])
	}

	// create a request to be sent to the execution node
	fwdReq := &access.GetEventsForBlockIDsRequest{
		Type:     reqEvent,
		BlockIds: blockIDs,
	}

	return h.executionRPC.GetEventsForBlockIDs(ctx, fwdReq)
}

// GetEventsForBlockIDs retrieves events for all the specified block IDs that have the given type
func (h *Handler) GetEventsForBlockIDs(ctx context.Context, req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {

	if len(req.GetBlockIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "block IDs not specified")
	}

	// forward the request to the execution node
	return h.executionRPC.GetEventsForBlockIDs(ctx, req)
}
