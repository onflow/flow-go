package backend

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendEvents struct {
	staticExecutionRPC execproto.ExecutionAPIClient
	blocks             storage.Blocks
	state              protocol.State
	connFactory        ConnectionFactory
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

	// find the block headers for all the blocks between min and max height (inclusive)
	blockHeaders := make([]*flow.Header, 0)

	for i := startHeight; i <= endHeight; i++ {
		block, err := b.blocks.ByHeight(i)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get events: %v", err)
		}

		blockHeaders = append(blockHeaders, block.Header)
	}

	return b.getBlockEventsFromExecutionNode(ctx, blockHeaders, eventType)
}

// GetEventsForBlockIDs retrieves events for all the specified block IDs that have the given type
func (b *backendEvents) GetEventsForBlockIDs(
	ctx context.Context,
	eventType string,
	blockIDs []flow.Identifier,
) ([]flow.BlockEvents, error) {

	// find the block headers for all the block IDs
	blockHeaders := make([]*flow.Header, 0)
	for _, blockID := range blockIDs {
		block, err := b.blocks.ByID(blockID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get events: %v", err)
		}

		blockHeaders = append(blockHeaders, block.Header)
	}

	// forward the request to the execution node
	return b.getBlockEventsFromExecutionNode(ctx, blockHeaders, eventType)
}

func (b *backendEvents) getBlockEventsFromExecutionNode(
	ctx context.Context,
	blockHeaders []*flow.Header,
	eventType string,
) ([]flow.BlockEvents, error) {

	// create an execution API request for events at block ID
	blockIDs := make([]flow.Identifier, len(blockHeaders))
	for i := range blockIDs {
		blockIDs[i] = blockHeaders[i].ID()
	}
	req := execproto.GetEventsForBlockIDsRequest{
		Type:     eventType,
		BlockIds: convert.IdentifiersToMessages(blockIDs),
	}

	// call the execution node gRPC
	resp, err := b.staticExecutionRPC.GetEventsForBlockIDs(ctx, &req)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve events from execution node: %v", err)
	}

	// convert execution node api result to access node api result
	results, err := verifyAndConvertToAccessEvents(resp.GetResults(), blockHeaders)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to verify retrieved events from execution node: %v", err)
	}

	return results, nil
}

// verifyAndConvertToAccessEvents converts execution node api result to access node api result, and verifies that the results contains
// results from each block that was requested
func verifyAndConvertToAccessEvents(execEvents []*execproto.GetEventsForBlockIDsResponse_Result, requestedBlockHeaders []*flow.Header) ([]flow.BlockEvents, error) {
	if len(execEvents) != len(requestedBlockHeaders) {
		return nil, errors.New("number of results does not match number of blocks requested")
	}

	reqestedBlockHeaderSet := map[string]*flow.Header{}
	for _, header := range requestedBlockHeaders {
		reqestedBlockHeaderSet[header.ID().String()] = header
	}

	results := make([]flow.BlockEvents, len(execEvents))

	for i, result := range execEvents {
		header, expected := reqestedBlockHeaderSet[hex.EncodeToString(result.GetBlockId())]
		if !expected {
			return nil, fmt.Errorf("unexpected blockID from exe node %x", result.GetBlockId())
		}
		if result.GetBlockHeight() != header.Height {
			return nil, fmt.Errorf("unexpected block height %d for block %x from exe node",
				result.GetBlockHeight(),
				result.GetBlockId())
		}

		results[i] = flow.BlockEvents{
			BlockID:        header.ID(),
			BlockHeight:    header.Height,
			BlockTimestamp: header.Timestamp,
			Events:         convert.MessagesToEvents(result.GetEvents()),
		}
	}

	return results, nil
}
