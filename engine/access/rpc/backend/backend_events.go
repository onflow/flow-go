package backend

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/dapperlabs/flow-go/engine/common/rpc/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

type backendEvents struct {
	executionRPC execution.ExecutionAPIClient
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
	req := execution.GetEventsForBlockIDsRequest{
		Type:     eventType,
		BlockIds: convert.IdentifiersToMessages(blockIDs),
	}

	// call the execution node gRPC
	resp, err := b.executionRPC.GetEventsForBlockIDs(ctx, &req)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve events from execution node: %v", err)
	}

	// convert execution node api result to access node api result
	results := accessEvents(resp.GetResults())

	return results, nil
}

// accessEvents converts execution node api result to access node api result
func accessEvents(execResults []*execution.GetEventsForBlockIDsResponse_Result) []flow.BlockEvents {
	results := make([]flow.BlockEvents, len(execResults))

	for i, result := range execResults {
		results[i] = flow.BlockEvents{
			BlockID:     convert.MessageToIdentifier(result.GetBlockId()),
			BlockHeight: result.GetBlockHeight(),
			Events:      convert.MessagesToEvents(result.GetEvents()),
		}
	}

	return results
}
