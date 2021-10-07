package backend

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendEvents struct {
	headers           storage.Headers
	executionReceipts storage.ExecutionReceipts
	state             protocol.State
	connFactory       ConnectionFactory
	log               zerolog.Logger
	maxHeightRange    uint
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

	rangeSize := endHeight - startHeight + 1 // range is inclusive on both ends
	if rangeSize > uint64(b.maxHeightRange) {
		return nil, status.Errorf(codes.InvalidArgument, "requested block range (%d) exceeded maximum (%d)", rangeSize, b.maxHeightRange)
	}

	// get the latest sealed block header
	head, err := b.state.Sealed().Head()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get events: %v", err)
	}

	// start height should not be beyond the last sealed height
	if head.Height < startHeight {
		return nil, status.Errorf(codes.OutOfRange,
			"start height %d is greater than the last sealed block height %d", startHeight, head.Height)
	}

	// limit max height to last sealed block in the chain
	if head.Height < endHeight {
		endHeight = head.Height
	}

	// find the block headers for all the blocks between min and max height (inclusive)
	blockHeaders := make([]*flow.Header, 0)

	for i := startHeight; i <= endHeight; i++ {
		header, err := b.headers.ByHeight(i)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get events: %v", err)
		}

		blockHeaders = append(blockHeaders, header)
	}

	return b.getBlockEventsFromExecutionNode(ctx, blockHeaders, eventType)
}

// GetEventsForBlockIDs retrieves events for all the specified block IDs that have the given type
func (b *backendEvents) GetEventsForBlockIDs(
	ctx context.Context,
	eventType string,
	blockIDs []flow.Identifier,
) ([]flow.BlockEvents, error) {

	if uint(len(blockIDs)) > b.maxHeightRange {
		return nil, fmt.Errorf("requested block range (%d) exceeded maximum (%d)", len(blockIDs), b.maxHeightRange)
	}

	// find the block headers for all the block IDs
	blockHeaders := make([]*flow.Header, 0)
	for _, blockID := range blockIDs {
		header, err := b.headers.ByBlockID(blockID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get events: %v", err)
		}

		blockHeaders = append(blockHeaders, header)
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

	if len(blockIDs) == 0 {
		return []flow.BlockEvents{}, nil
	}

	req := execproto.GetEventsForBlockIDsRequest{
		Type:     eventType,
		BlockIds: convert.IdentifiersToMessages(blockIDs),
	}

	// choose the last block ID to find the list of execution nodes
	lastBlockID := blockIDs[len(blockIDs)-1]

	execNodes, err := executionNodesForBlockID(ctx, lastBlockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve events from execution node: %v", err)
	}

	var resp *execproto.GetEventsForBlockIDsResponse
	var successfulNode *flow.Identity
	resp, successfulNode, err = b.getEventsFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve events from execution nodes %s: %v", execNodes, err)
	}
	b.log.Trace().
		Str("execution_id", successfulNode.String()).
		Str("last_block_id", lastBlockID.String()).
		Msg("successfully got events")

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

func (b *backendEvents) getEventsFromAnyExeNode(ctx context.Context,
	execNodes flow.IdentityList,
	req execproto.GetEventsForBlockIDsRequest) (*execproto.GetEventsForBlockIDsResponse, *flow.Identity, error) {
	var errors *multierror.Error
	// try to get events from one of the execution nodes
	for _, execNode := range execNodes {
		resp, err := b.tryGetEvents(ctx, execNode, req)
		if err == nil {
			return resp, execNode, nil
		}
		errors = multierror.Append(errors, err)
	}
	return nil, nil, errors.ErrorOrNil()
}

func (b *backendEvents) tryGetEvents(ctx context.Context,
	execNode *flow.Identity,
	req execproto.GetEventsForBlockIDsRequest) (*execproto.GetEventsForBlockIDsResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	resp, err := execRPCClient.GetEventsForBlockIDs(ctx, &req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
