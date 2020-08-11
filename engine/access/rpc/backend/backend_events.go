package backend

import (
	"context"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/engine/common/rpc/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

type backendEvents struct {
	executionRPC execproto.ExecutionAPIClient
	blocks       storage.Blocks
	state        protocol.State
}

// GetEventsForHeightRange retrieves events for all sealed blocks between the start block height and
// the end block height (inclusive) that have the given type.
func (b *backendEvents) GetEventsForHeightRange(
	ctx context.Context,
	eventType string,
	startHeight, endHeight uint64,
) ([]flow.BlockEvents, error) {

	if endHeight < startHeight {
		return nil, status.Error(codes.InvalidArgument, "invalid start or end height")
	}

	// get the latest sealed block header
	head, err := b.state.Sealed().Head()
	if err != nil {
		return nil, status.Errorf(codes.Internal, " failed to get events: %v", err)
	}

	// limit max height to last sealed block in the chain
	if head.Height < endHeight {
		endHeight = head.Height
	}

	// find the block IDs for all the blocks between min and max height (inclusive)
	blockIDs := make([]flow.Identifier, 0)

	for i := startHeight; i <= endHeight; i++ {
		block, err := b.blocks.ByHeight(i)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get events: %v", err)
		}

		blockIDs = append(blockIDs, block.ID())
	}

	return b.getBlockEventsFromExecutionNode(ctx, blockIDs, eventType)
}

// GetEventsForBlockIDs retrieves events for all the specified block IDs that have the given type
func (b *backendEvents) GetEventsForBlockIDs(
	ctx context.Context,
	eventType string,
	blockIDs []flow.Identifier,
) ([]flow.BlockEvents, error) {
	// forward the request to the execution node
	return b.getBlockEventsFromExecutionNode(ctx, blockIDs, eventType)
}

func (b *backendEvents) getBlockEventsFromExecutionNode(
	ctx context.Context,
	blockIDs []flow.Identifier,
	eventType string,
) ([]flow.BlockEvents, error) {

	// create an execution API request for events at block ID
	req := execproto.GetEventsForBlockIDsRequest{
		Type:     eventType,
		BlockIds: convert.IdentifiersToMessages(blockIDs),
	}

	// call the execution node gRPC
	resp, err := b.executionRPC.GetEventsForBlockIDs(ctx, &req)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve events from execution node: %v", err)
	}

	// convert execution node api result to access node api result
	results, err := verifyAndConvertToAccessEvents(resp.GetResults(), blockIDs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to verify retrieved events from execution node: %v", err)
	}
	
	return results, nil
}

// verifyAndConvertToAccessEvents converts execution node api result to access node api result, and verifies that the results contains
// results from each block that was requested
func verifyAndConvertToAccessEvents(execEvents []*execution.GetEventsForBlockIDsResponse_Result, requestedBlockIDs [][]byte) ([]*access.EventsResponse_Result, error) {
	if len(execEvents) != len(requestedBlockIDs) {
		return nil, errors.New("number of results does not match number of blocks requested")
	}

	blockIDSet := map[string]bool{}
	for _, blockID := range requestedBlockIDs {
		blockIDSet[string(blockID)] = true
	}

	results := make([]*access.EventsResponse_Result, len(execEvents))

	for i, r := range execEvents {
		if !blockIDSet[string(r.GetBlockId())] {
			return nil, fmt.Errorf("unexpected blockID from exe node %x", r.GetBlockId())
		}

		results[i] = &access.EventsResponse_Result{
			BlockId:     r.GetBlockId(),
			BlockHeight: r.GetBlockHeight(),
			Events:      r.GetEvents(),
		}
	}

	return results, nil
}
